package c

/*
#cgo CFLAGS: -I./agent/internal/c
#cgo LDFLAGS: -lpthread
#include "lockfree_queue.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"sync"
	"unsafe"
)

// QueueErrors
var (
	ErrQueueClosed = &QueueError{"queue is closed"}
	ErrQueueFull   = &QueueError{"queue is full"}
	ErrQueueEmpty  = &QueueError{"queue is empty"}
)

// QueueError 队列错误
type QueueError struct {
	msg string
}

func (e *QueueError) Error() string {
	return e.msg
}

// LockFreeQueue 无锁队列包装器
type LockFreeQueue struct {
	ptr    *C.lockfree_queue_t
	mu     sync.Mutex
	closed bool
}

// NewLockFreeQueue 创建新的无锁队列
func NewLockFreeQueue(capacity int) *LockFreeQueue {
	lq := &LockFreeQueue{
		ptr: C.lfq_create(C.size_t(capacity)),
	}
	if lq.ptr == nil {
		return nil
	}
	return lq
}

// Enqueue 添加数据到队列
func (lq *LockFreeQueue) Enqueue(data unsafe.Pointer) error {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return ErrQueueClosed
	}

	var item C.lfq_connection_t
	item.fd = 0  // 这里我们只是存储数据指针
	item.data = data

	result := C.lfq_enqueue(lq.ptr, &item)
	if result != 0 {
		return ErrQueueFull
	}

	return nil
}

// Dequeue 从队列获取数据
func (lq *LockFreeQueue) Dequeue() (unsafe.Pointer, error) {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return nil, ErrQueueClosed
	}

	var item C.lfq_connection_t
	result := C.lfq_dequeue(lq.ptr, &item)

	if result != 0 {
		return nil, ErrQueueEmpty
	}

	return item.data, nil
}

// Size 获取当前大小
func (lq *LockFreeQueue) Size() int {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return 0
	}

	return int(C.lfq_size(lq.ptr))
}

// IsEmpty 检查是否为空
func (lq *LockFreeQueue) IsEmpty() bool {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return true
	}

	return C.lfq_is_empty(lq.ptr) == 1
}

// Close 关闭队列
func (lq *LockFreeQueue) Close() {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if lq.closed {
		return
	}

	C.lfq_destroy(lq.ptr)
	lq.ptr = nil
	lq.closed = true
}
