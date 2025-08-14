package cpp

/*
#cgo CXXFLAGS: -I. -std=c++17
#cgo LDFLAGS: -L. -lstreaming_topk -lstdc++ -lpthread
#include <stdlib.h>
#include <string.h>
#include "streaming_topk_c_wrapper.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

// StreamingTopKItem Go版本的流式Top-K项
type StreamingTopKItem struct {
	HostID      string  `json:"host_id"`
	Value       float64 `json:"value"`
	Timestamp   int64   `json:"timestamp"`
	Index       int32   `json:"index"`
	UpdateCount int32   `json:"update_count"`
}

// StreamingTopKProcessor Go版本的流式Top-K处理器
type StreamingTopKProcessor struct {
	ptr    C.StreamingTopKProcessor_Handle
	mu     sync.Mutex
	closed bool
}

// NewStreamingTopKProcessor 创建新的流式Top-K处理器
func NewStreamingTopKProcessor(k int, ttlSeconds int) *StreamingTopKProcessor {
	handle := C.StreamingTopKProcessor_Create(C.int(k), C.int(ttlSeconds))
	if handle == nil {
		return nil
	}

	return &StreamingTopKProcessor{
		ptr: handle,
	}
}

// SetUpdateCallback 设置更新回调
func (p *StreamingTopKProcessor) SetUpdateCallback(callback func([]StreamingTopKItem)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	// 这里需要实现C++回调到Go回调的桥接
	// 由于CGO的限制，这里简化处理
	// 实际实现需要更复杂的回调机制
}

// SetExpireCallback 设置过期回调
func (p *StreamingTopKProcessor) SetExpireCallback(callback func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	// 这里需要实现C++回调到Go回调的桥接
}

// ProcessStream 流式处理单个数据
func (p *StreamingTopKProcessor) ProcessStream(hostID string, value float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrProcessorClosed
	}

	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))

	result := C.StreamingTopKProcessor_ProcessStream(p.ptr, cHostID, C.double(value))
	if result != 0 {
		return &StreamingTopKError{"failed to process stream"}
	}

	return nil
}

// ProcessStreamBatch 批量流式处理
func (p *StreamingTopKProcessor) ProcessStreamBatch(items []struct {
	HostID string
	Value  float64
}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrProcessorClosed
	}

	// 转换为C结构
	cItems := make([]C.StreamingTopKItem_C, len(items))
	for i, item := range items {
		cHostID := C.CString(item.HostID)
		defer C.free(unsafe.Pointer(cHostID))

		cItems[i] = C.StreamingTopKItem_C{
			host_id: [64]C.char{},
			value:   C.double(item.Value),
		}
		C.strcpy(&cItems[i].host_id[0], cHostID)
	}

	result := C.StreamingTopKProcessor_ProcessStreamBatch(p.ptr, (*C.StreamingTopKItem_C)(unsafe.Pointer(&cItems[0])), C.int(len(items)))
	if result != 0 {
		return &StreamingTopKError{"failed to process stream batch"}
	}

	return nil
}

// GetTopK 获取当前Top-K结果
func (p *StreamingTopKProcessor) GetTopK() []StreamingTopKItem {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	// 调用C++函数获取结果
	result := C.StreamingTopKProcessor_GetTopK(p.ptr)
	if result == nil {
		return nil
	}

	// 转换为Go slice
	goItems := make([]StreamingTopKItem, result.count)
	cItems := (*[1 << 30]C.StreamingTopKItem_C)(unsafe.Pointer(result.items))[:result.count:result.count]

	for i, cItem := range cItems {
		goItems[i] = StreamingTopKItem{
			HostID:      C.GoString(&cItem.host_id[0]),
			Value:       float64(cItem.value),
			Timestamp:   int64(cItem.timestamp),
			Index:       int32(cItem.index),
			UpdateCount: int32(cItem.update_count),
		}
	}

	// 释放C++分配的内存
	C.StreamingTopKProcessor_FreeResult(result)

	return goItems
}

// GetStats 获取统计信息
func (p *StreamingTopKProcessor) GetStats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	stats := C.StreamingTopKProcessor_GetStats(p.ptr)
	if stats == nil {
		return nil
	}

	goStats := map[string]interface{}{
		"total_updates": int64(stats.total_updates),
		"current_items": int(stats.current_items),
		"k_value":       int(stats.k_value),
		"ttl_seconds":   int(stats.ttl_seconds),
		"is_closed":     stats.is_closed != 0,
	}

	C.StreamingTopKProcessor_FreeStats(stats)

	return goStats
}

// GetCount 获取当前元素数量
func (p *StreamingTopKProcessor) GetCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0
	}

	return int(C.StreamingTopKProcessor_GetCount(p.ptr))
}

// Contains 检查是否包含指定主机
func (p *StreamingTopKProcessor) Contains(hostID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return false
	}

	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))

	return C.StreamingTopKProcessor_Contains(p.ptr, cHostID) != 0
}

// GetValue 获取指定主机的值
func (p *StreamingTopKProcessor) GetValue(hostID string) (float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0, false
	}

	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))

	var value C.double
	result := C.StreamingTopKProcessor_GetValue(p.ptr, cHostID, &value)

	return float64(value), result != 0
}

// Clear 清空所有数据
func (p *StreamingTopKProcessor) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	C.StreamingTopKProcessor_Clear(p.ptr)
}

// SetTTL 设置TTL
func (p *StreamingTopKProcessor) SetTTL(seconds int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	C.StreamingTopKProcessor_SetTTL(p.ptr, C.int(seconds))
}

// GetTTL 获取TTL
func (p *StreamingTopKProcessor) GetTTL() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0
	}

	return int(C.StreamingTopKProcessor_GetTTL(p.ptr))
}

// GetK 获取K值
func (p *StreamingTopKProcessor) GetK() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0
	}

	return int(C.StreamingTopKProcessor_GetK(p.ptr))
}

// SetK 设置K值
func (p *StreamingTopKProcessor) SetK(k int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	C.StreamingTopKProcessor_SetK(p.ptr, C.int(k))
}

// Close 关闭处理器
func (p *StreamingTopKProcessor) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	C.StreamingTopKProcessor_Close(p.ptr)
	p.ptr = nil
	p.closed = true
}

// IsClosed 检查是否已关闭
func (p *StreamingTopKProcessor) IsClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.closed
}

// StreamingTopKManager Go版本的流式Top-K管理器
type StreamingTopKManager struct {
	ptr    C.StreamingTopKManager_Handle
	mu     sync.Mutex
	closed bool
}

// GetStreamingTopKManagerInstance 获取流式Top-K管理器实例
func GetStreamingTopKManagerInstance() *StreamingTopKManager {
	handle := C.StreamingTopKManager_GetInstance()
	if handle == nil {
		return nil
	}

	return &StreamingTopKManager{
		ptr: handle,
	}
}

// Initialize 初始化管理器
func (m *StreamingTopKManager) Initialize(defaultK, defaultTTL int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrManagerClosed
	}

	result := C.StreamingTopKManager_Initialize(m.ptr, C.int(defaultK), C.int(defaultTTL))
	if result != 0 {
		return &StreamingTopKError{"failed to initialize manager"}
	}

	return nil
}

// CreateProcessor 创建新的处理器
func (m *StreamingTopKManager) CreateProcessor(name string, k, ttlSeconds int) (*StreamingTopKProcessor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, ErrManagerClosed
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.StreamingTopKManager_CreateProcessor(m.ptr, cName, C.int(k), C.int(ttlSeconds))
	if handle == nil {
		return nil, &StreamingTopKError{"failed to create processor"}
	}

	return &StreamingTopKProcessor{
		ptr: handle,
	}, nil
}

// GetProcessor 获取处理器
func (m *StreamingTopKManager) GetProcessor(name string) *StreamingTopKProcessor {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.StreamingTopKManager_GetProcessor(m.ptr, cName)
	if handle == nil {
		return nil
	}

	return &StreamingTopKProcessor{
		ptr: handle,
	}
}

// RemoveProcessor 移除处理器
func (m *StreamingTopKManager) RemoveProcessor(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return false
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	return C.StreamingTopKManager_RemoveProcessor(m.ptr, cName) != 0
}

// AddMetric 添加指标数据
func (m *StreamingTopKManager) AddMetric(processorName, hostID string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrManagerClosed
	}

	cProcessorName := C.CString(processorName)
	defer C.free(unsafe.Pointer(cProcessorName))

	cHostID := C.CString(hostID)
	defer C.free(unsafe.Pointer(cHostID))

	result := C.StreamingTopKManager_AddMetric(m.ptr, cProcessorName, cHostID, C.double(value))
	if result != 0 {
		return &StreamingTopKError{"failed to add metric"}
	}

	return nil
}

// GetTopK 获取Top-K结果
func (m *StreamingTopKManager) GetTopK(processorName string) []StreamingTopKItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	cProcessorName := C.CString(processorName)
	defer C.free(unsafe.Pointer(cProcessorName))

	result := C.StreamingTopKManager_GetTopK(m.ptr, cProcessorName)
	if result == nil {
		return nil
	}

	// 转换为Go slice
	goItems := make([]StreamingTopKItem, result.count)
	cItems := (*[1 << 30]C.StreamingTopKItem_C)(unsafe.Pointer(result.items))[:result.count:result.count]

	for i, cItem := range cItems {
		goItems[i] = StreamingTopKItem{
			HostID:      C.GoString(&cItem.host_id[0]),
			Value:       float64(cItem.value),
			Timestamp:   int64(cItem.timestamp),
			Index:       int32(cItem.index),
			UpdateCount: int32(cItem.update_count),
		}
	}

	C.StreamingTopKManager_FreeResult(result)

	return goItems
}

// GetProcessorNames 获取所有处理器名称
func (m *StreamingTopKManager) GetProcessorNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	result := C.StreamingTopKManager_GetProcessorNames(m.ptr)
	if result == nil {
		return nil
	}

	names := make([]string, result.count)
	cNames := (*[1 << 30]*C.char)(unsafe.Pointer(result.names))[:result.count:result.count]

	for i, cName := range cNames {
		names[i] = C.GoString(cName)
	}

	C.StreamingTopKManager_FreeNames(result)

	return names
}

// GetSystemStats 获取系统统计信息
func (m *StreamingTopKManager) GetSystemStats() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	stats := C.StreamingTopKManager_GetSystemStats(m.ptr)
	if stats == nil {
		return nil
	}

	goStats := map[string]interface{}{
		"total_processors": int(stats.total_processors),
		"initialized":      stats.initialized != 0,
		"default_k":        int(stats.default_k),
		"default_ttl":      int(stats.default_ttl),
		"running":          stats.running != 0,
		"total_updates":    int64(stats.total_updates),
		"total_items":      int(stats.total_items),
	}

	C.StreamingTopKManager_FreeStats(stats)

	return goStats
}

// Shutdown 关闭管理器
func (m *StreamingTopKManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	C.StreamingTopKManager_Shutdown(m.ptr)
	m.closed = true
}

// 错误类型
var (
	ErrProcessorClosed = &StreamingTopKError{"processor is closed"}
	ErrManagerClosed   = &StreamingTopKError{"manager is closed"}
)

type StreamingTopKError struct {
	msg string
}

func (e *StreamingTopKError) Error() string {
	return e.msg
}
