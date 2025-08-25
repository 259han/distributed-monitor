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
	"sync/atomic"
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
	closed int64 // 使用原子变量替代锁，0=open, 1=closed
}

// NewLockFreeQueue 创建新的无锁队列
func NewLockFreeQueue(capacity int) *LockFreeQueue {
	lq := &LockFreeQueue{
		ptr:    C.lfq_create(C.size_t(capacity)),
		closed: 0, // 初始状态为开放
	}
	if lq.ptr == nil {
		return nil
	}
	return lq
}

// Enqueue 添加数据到队列（真正无锁）
func (lq *LockFreeQueue) Enqueue(data unsafe.Pointer) error {
	// 使用原子操作检查关闭状态
	if atomic.LoadInt64(&lq.closed) != 0 {
		return ErrQueueClosed
	}

	var item C.lfq_connection_t
	item.fd = 0 // 这里我们只是存储数据指针
	item.data = data

	result := C.lfq_enqueue_conn(lq.ptr, &item)
	if result != 0 {
		return ErrQueueFull
	}

	return nil
}

// Dequeue 从队列获取数据（真正无锁）
func (lq *LockFreeQueue) Dequeue() (unsafe.Pointer, error) {
	// 使用原子操作检查关闭状态
	if atomic.LoadInt64(&lq.closed) != 0 {
		return nil, ErrQueueClosed
	}

	var item C.lfq_connection_t
	result := C.lfq_dequeue(lq.ptr, &item)

	if result != 0 {
		return nil, ErrQueueEmpty
	}

	return item.data, nil
}

// Size 获取当前大小（无锁）
func (lq *LockFreeQueue) Size() int {
	if atomic.LoadInt64(&lq.closed) != 0 {
		return 0
	}

	return int(C.lfq_size(lq.ptr))
}

// IsEmpty 检查是否为空（无锁）
func (lq *LockFreeQueue) IsEmpty() bool {
	if atomic.LoadInt64(&lq.closed) != 0 {
		return true
	}

	return C.lfq_is_empty(lq.ptr) == 1
}

// Close 关闭队列（使用原子操作）
func (lq *LockFreeQueue) Close() {
	// 使用CAS原子操作设置关闭状态，确保只关闭一次
	if !atomic.CompareAndSwapInt64(&lq.closed, 0, 1) {
		return // 已经关闭
	}

	if lq.ptr != nil {
		C.lfq_destroy(lq.ptr)
		C.free(unsafe.Pointer(lq.ptr))
		lq.ptr = nil
	}
}

// IsClosed 检查队列是否已关闭
func (lq *LockFreeQueue) IsClosed() bool {
	return atomic.LoadInt64(&lq.closed) != 0
}
