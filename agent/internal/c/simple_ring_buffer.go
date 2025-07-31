package c

/*
#cgo CFLAGS: -I./agent/internal/c
#cgo LDFLAGS: -lpthread
#include "ring_buffer.h"
#include <stdlib.h>
#include <string.h>

// 重新声明一些C函数以便在Go中使用
static void go_strcpy(char *dest, const char *src) {
    strcpy(dest, src);
}
*/
import "C"

import (
	"sync"
	"unsafe"
)

// Errors
var (
	ErrBufferClosed = &BufferError{"buffer is closed"}
	ErrBufferFull   = &BufferError{"buffer is full"}
	ErrBufferEmpty  = &BufferError{"buffer is empty"}
)

// BufferError ring buffer错误
type BufferError struct {
	msg string
}

func (e *BufferError) Error() string {
	return e.msg
}

// SimpleMetricsRingBuffer 简化的MetricsData ring buffer包装器
type SimpleMetricsRingBuffer struct {
	ptr    *C.ring_buffer_t
	mu     sync.Mutex
	closed bool
}

// NewSimpleMetricsRingBuffer 创建新的简化ring buffer
func NewSimpleMetricsRingBuffer(size int, blocking bool) *SimpleMetricsRingBuffer {
	var blockingInt C.int
	if blocking {
		blockingInt = 1
	} else {
		blockingInt = 0
	}

	rb := &SimpleMetricsRingBuffer{
		ptr: C.ring_buffer_create(C.size_t(size), blockingInt),
	}
	if rb.ptr == nil {
		return nil
	}
	return rb
}

// PushData 添加数据指针到ring buffer
func (rb *SimpleMetricsRingBuffer) PushData(data unsafe.Pointer) error {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return ErrBufferClosed
	}

	result := C.ring_buffer_push(rb.ptr, data)
	if result != 0 {
		return ErrBufferFull
	}

	return nil
}

// PopData 从ring buffer获取数据指针
func (rb *SimpleMetricsRingBuffer) PopData() (unsafe.Pointer, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return nil, ErrBufferClosed
	}

	var dataPtr unsafe.Pointer
	result := C.ring_buffer_pop(rb.ptr, &dataPtr)

	if result != 0 {
		return nil, ErrBufferEmpty
	}

	return dataPtr, nil
}

// TryPopData 非阻塞尝试获取数据指针
func (rb *SimpleMetricsRingBuffer) TryPopData() (unsafe.Pointer, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return nil, ErrBufferClosed
	}

	var dataPtr unsafe.Pointer
	result := C.ring_buffer_try_pop(rb.ptr, &dataPtr)

	if result != 0 {
		return nil, ErrBufferEmpty
	}

	return dataPtr, nil
}

// Size 获取当前大小
func (rb *SimpleMetricsRingBuffer) Size() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return 0
	}

	return int(C.ring_buffer_size(rb.ptr))
}

// Capacity 获取当前容量
func (rb *SimpleMetricsRingBuffer) Capacity() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return 0
	}

	return int(C.ring_buffer_capacity(rb.ptr))
}

// IsEmpty 检查是否为空
func (rb *SimpleMetricsRingBuffer) IsEmpty() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return true
	}

	return C.ring_buffer_is_empty(rb.ptr) == 1
}

// Clear 清空ring buffer
func (rb *SimpleMetricsRingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return
	}

	C.ring_buffer_clear(rb.ptr)
}

// Close 关闭ring buffer
func (rb *SimpleMetricsRingBuffer) Close() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.closed {
		return
	}

	C.ring_buffer_destroy(rb.ptr)
	rb.ptr = nil
	rb.closed = true
}
