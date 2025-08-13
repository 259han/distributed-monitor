package cpp

/*
#cgo CXXFLAGS: -I./visualization/internal/cpp
#cgo LDFLAGS: -L./bin/lib -ltopk -lrt -lpthread
#cgo LDFLAGS: -lstdc++
#include <stdlib.h>
#include <string.h>
#include "topk_c_wrapper.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

// TopKManager C++ Top-K管理器的Go包装器
type TopKManager struct {
	ptr     C.FastTopK_Handle
	mu      sync.Mutex
	closed  bool
}

// NewTopKManager 创建新的Top-K管理器
func NewTopKManager(k int, ttlSeconds int) *TopKManager {
	handle := C.FastTopK_Create(C.int(k), C.int(ttlSeconds))
	if handle == nil {
		return nil
	}
	
	return &TopKManager{
		ptr: handle,
	}
}

// Add 添加或更新元素
func (tm *TopKManager) Add(hostID string, value float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return ErrManagerClosed
	}
	
	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))
	
	result := C.FastTopK_Add(tm.ptr, cHostID, C.double(value))
	if result != 0 {
		return &TopKError{"failed to add item"}
	}
	
	return nil
}

// GetTopK 获取Top-K元素
func (tm *TopKManager) GetTopK() []TopKItem {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return nil
	}
	
	// 调用C++函数获取结果
	result := C.FastTopK_GetTopK(tm.ptr)
	if result == nil {
		return nil
	}
	
	// 转换为Go slice
	goItems := make([]TopKItem, result.count)
	cItems := (*[1 << 30]C.TopKItem_C)(unsafe.Pointer(result.items))[:result.count:result.count]
	
	for i, cItem := range cItems {
		goItems[i] = TopKItem{
			HostID:    C.GoString(&cItem.host_id[0]),
			Value:     float64(cItem.value),
			Timestamp: int64(cItem.timestamp),
		}
	}
	
	// 释放C++分配的内存
	C.FastTopK_FreeResult(result)
	
	return goItems
}

// GetCount 获取当前元素数量
func (tm *TopKManager) GetCount() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return 0
	}
	
	return int(C.FastTopK_GetCount(tm.ptr))
}

// Clear 清空Top-K
func (tm *TopKManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return
	}
	
	C.FastTopK_Clear(tm.ptr)
}

// SetTTL 设置TTL
func (tm *TopKManager) SetTTL(seconds int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return
	}
	
	C.FastTopK_SetTTL(tm.ptr, C.int(seconds))
}

// GetTTL 获取当前TTL
func (tm *TopKManager) GetTTL() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return 0
	}
	
	return int(C.FastTopK_GetTTL(tm.ptr))
}

// GetK 获取K值
func (tm *TopKManager) GetK() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return 0
	}
	
	return int(C.FastTopK_GetK(tm.ptr))
}

// SetK 设置K值
func (tm *TopKManager) SetK(k int) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return ErrManagerClosed
	}
	
	result := C.FastTopK_SetK(tm.ptr, C.int(k))
	if result != 0 {
		return &TopKError{"failed to set K value"}
	}
	
	return nil
}

// GetValue 获取特定主机的值
func (tm *TopKManager) GetValue(hostID string) (float64, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return 0, false
	}
	
	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))
	
	value := C.FastTopK_GetValue(tm.ptr, cHostID)
	return float64(value), value != -1.0
}

// Remove 移除特定主机
func (tm *TopKManager) Remove(hostID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return ErrManagerClosed
	}
	
	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))
	
	result := C.FastTopK_Remove(tm.ptr, cHostID)
	if result != 0 {
		return &TopKError{"failed to remove item"}
	}
	
	return nil
}

// Contains 检查是否包含特定主机
func (tm *TopKManager) Contains(hostID string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return false
	}
	
	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))
	
	return C.FastTopK_Contains(tm.ptr, cHostID) == 1
}

// GetExpiredCount 获取过期项目数量
func (tm *TopKManager) GetExpiredCount() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return 0
	}
	
	return int(C.FastTopK_GetExpiredCount(tm.ptr))
}

// CleanupExpired 清理过期项目
func (tm *TopKManager) CleanupExpired() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return 0
	}
	
	return int(C.FastTopK_CleanupExpired(tm.ptr))
}

// GetStats 获取统计信息
type TopKStats struct {
	TotalItems      int64
	ActiveItems     int
	ExpiredItems    int
	TotalOperations int64
	Uptime          int64
	MemoryUsage     int64
}

// GetStats 获取统计信息
func (tm *TopKManager) GetStats() *TopKStats {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return nil
	}
	
	cStats := C.FastTopK_GetStats(tm.ptr)
	if cStats == nil {
		return nil
	}
	
	stats := &TopKStats{
		TotalItems:      int64(cStats.total_items),
		ActiveItems:     int(cStats.active_items),
		ExpiredItems:    int(cStats.expired_items),
		TotalOperations: int64(cStats.total_operations),
		Uptime:          int64(cStats.uptime),
		MemoryUsage:     int64(cStats.memory_usage),
	}
	
	// 释放C++分配的内存
	C.FastTopK_FreeStats(cStats)
	
	return stats
}

// GetTopKFiltered 获取过滤后的Top-K结果
func (tm *TopKManager) GetTopKFiltered(minValue, maxValue float64) []TopKItem {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return nil
	}
	
	result := C.FastTopK_GetTopKFiltered(tm.ptr, C.double(minValue), C.double(maxValue))
	if result == nil {
		return nil
	}
	
	// 转换为Go slice
	goItems := make([]TopKItem, result.count)
	cItems := (*[1 << 30]C.TopKItem_C)(unsafe.Pointer(result.items))[:result.count:result.count]
	
	for i, cItem := range cItems {
		goItems[i] = TopKItem{
			HostID:    C.GoString(&cItem.host_id[0]),
			Value:     float64(cItem.value),
			Timestamp: int64(cItem.timestamp),
		}
	}
	
	// 释放C++分配的内存
	C.FastTopK_FreeResult(result)
	
	return goItems
}

// GetTopKByHostIDPattern 获取符合主机ID模式的Top-K结果
func (tm *TopKManager) GetTopKByHostIDPattern(pattern string) []TopKItem {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return nil
	}
	
	cPattern := C.CString(pattern)
	defer C.free(unsafe.Pointer(cPattern))
	
	result := C.FastTopK_GetTopKByHostIDPattern(tm.ptr, cPattern)
	if result == nil {
		return nil
	}
	
	// 转换为Go slice
	goItems := make([]TopKItem, result.count)
	cItems := (*[1 << 30]C.TopKItem_C)(unsafe.Pointer(result.items))[:result.count:result.count]
	
	for i, cItem := range cItems {
		goItems[i] = TopKItem{
			HostID:    C.GoString(&cItem.host_id[0]),
			Value:     float64(cItem.value),
			Timestamp: int64(cItem.timestamp),
		}
	}
	
	// 释放C++分配的内存
	C.FastTopK_FreeResult(result)
	
	return goItems
}

// ExportToJSON 导出为JSON字符串
func (tm *TopKManager) ExportToJSON() (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return "", ErrManagerClosed
	}
	
	jsonStr := C.FastTopK_ExportToJSON(tm.ptr)
	if jsonStr == nil {
		return "", &TopKError{"failed to export to JSON"}
	}
	
	defer C.free(unsafe.Pointer(jsonStr))
	
	return C.GoString(jsonStr), nil
}

// ImportFromJSON 从JSON字符串导入
func (tm *TopKManager) ImportFromJSON(jsonStr string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return ErrManagerClosed
	}
	
	cJSON := C.CString(jsonStr)
	defer C.free(unsafe.Pointer(cJSON))
	
	result := C.FastTopK_ImportFromJSON(tm.ptr, cJSON)
	if result != 0 {
		return &TopKError{"failed to import from JSON"}
	}
	
	return nil
}

// EnableDebug 启用调试模式
func (tm *TopKManager) EnableDebug(enable bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return
	}
	
	var enableInt C.int
	if enable {
		enableInt = 1
	} else {
		enableInt = 0
	}
	
	C.FastTopK_EnableDebug(tm.ptr, enableInt)
}

// Close 关闭Top-K管理器
func (tm *TopKManager) Close() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if tm.closed {
		return
	}
	
	C.FastTopK_Destroy(tm.ptr)
	tm.ptr = nil
	tm.closed = true
}

// TopKItem Top-K项
type TopKItem struct {
	HostID    string  // 主机ID
	Value     float64 // 指标值
	Timestamp int64   // 时间戳
}

// Errors
var (
	ErrManagerClosed = &TopKError{"manager is closed"}
)

// TopKError Top-K错误
type TopKError struct {
	msg string
}

func (e *TopKError) Error() string {
	return e.msg
}