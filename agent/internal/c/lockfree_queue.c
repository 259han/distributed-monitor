#include "lockfree_queue.h"
#include <string.h>
#include <stdlib.h>

lockfree_queue_t* lfq_create(size_t capacity) {
    if (capacity == 0 || capacity > LFQ_MAX_CONNECTIONS) {
        return NULL;
    }
    
    lockfree_queue_t* queue = (lockfree_queue_t*)malloc(sizeof(lockfree_queue_t));
    if (queue == NULL) {
        return NULL;
    }
    
    if (lfq_init(queue, capacity) != 0) {
        free(queue);
        return NULL;
    }
    
    return queue;
}

int lfq_init(lockfree_queue_t* queue, size_t capacity) {
    if (queue == NULL || capacity == 0 || capacity > LFQ_MAX_CONNECTIONS) {
        return -1;
    }

    memset(queue, 0, sizeof(lockfree_queue_t));
    queue->capacity = capacity;
    atomic_init(&queue->head, 0);
    atomic_init(&queue->tail, 0);
    atomic_init(&queue->initialized, 1);

    for (size_t i = 0; i < capacity; i++) {
        atomic_init(&queue->connections[i].state, 0);
        queue->connections[i].fd = -1;
        queue->connections[i].data = NULL;
    }

    return 0;
}

void lfq_destroy(lockfree_queue_t* queue) {
    if (queue == NULL || atomic_load(&queue->initialized) == 0) {
        return;
    }

    // 清理队列中残留的数据指针（主要是malloc的地址副本）
    size_t head = atomic_load(&queue->head);
    size_t tail = atomic_load(&queue->tail);
    
    while (head != tail) {
        if (queue->connections[head].data != NULL) {
            // 注意：这里不应该free data，因为data的内存管理由Go负责
            queue->connections[head].data = NULL;
        }
        head = (head + 1) % queue->capacity;
    }

    atomic_store(&queue->initialized, 0);
    queue->capacity = 0;
    atomic_store(&queue->head, 0);
    atomic_store(&queue->tail, 0);
    
    // 如果queue是通过lfq_create创建的，需要释放queue本身
    // 但这里无法确定，所以需要调用者明确调用free
}

int lfq_enqueue(lockfree_queue_t* queue, int fd, void* data) {
    if (queue == NULL || atomic_load(&queue->initialized) == 0 || fd < 0) {
        return -1;
    }

    size_t tail;
    size_t next_tail;
    size_t head;
    
    do {
        tail = atomic_load(&queue->tail);
        head = atomic_load(&queue->head);
        next_tail = (tail + 1) % queue->capacity;

        // 检查队列是否已满
        if (next_tail == head) {
            return -1; // Queue is full
        }

        // 尝试推进tail指针
        if (atomic_compare_exchange_weak(&queue->tail, &tail, next_tail)) {
            // 成功获得slot，写入数据
            queue->connections[tail].fd = fd;
            queue->connections[tail].data = data;
            // 使用release语义确保数据写入完成后再设置状态
            atomic_store_explicit(&queue->connections[tail].state, 1, memory_order_release);
            return 0;
        }
        // CAS失败，避免busy wait，让出CPU
        __asm__ __volatile__("pause" ::: "memory");
    } while (1);
}

int lfq_enqueue_conn(lockfree_queue_t* queue, lfq_connection_t* conn) {
    if (queue == NULL || atomic_load(&queue->initialized) == 0 || conn == NULL) {
        return -1;
    }
    
    return lfq_enqueue(queue, conn->fd, conn->data);
}

int lfq_dequeue(lockfree_queue_t* queue, lfq_connection_t* conn) {
    if (queue == NULL || atomic_load(&queue->initialized) == 0 || conn == NULL) {
        return -1;
    }

    size_t head;
    size_t next_head;
    size_t tail;
    
    do {
        head = atomic_load(&queue->head);
        tail = atomic_load(&queue->tail);
        
        if (head == tail) {
            return -1; // Queue is empty
        }

        next_head = (head + 1) % queue->capacity;

        // 使用acquire语义确保读取到完整的数据
        int state = atomic_load_explicit(&queue->connections[head].state, memory_order_acquire);
        if (state != 1) {
            // 数据还未完全写入，避免busy wait，让出CPU
            __asm__ __volatile__("pause" ::: "memory");
            continue;
        }

        if (atomic_compare_exchange_weak(&queue->head, &head, next_head)) {
            // 成功获得数据，复制并清理
            conn->fd = queue->connections[head].fd;
            conn->data = queue->connections[head].data;
            conn->state = state;
            
            // 清理slot
            atomic_store(&queue->connections[head].state, 0);
            queue->connections[head].fd = -1;
            queue->connections[head].data = NULL;
            return 0;
        }
        // CAS失败，重试
    } while (1);
}

int lfq_is_empty(lockfree_queue_t* queue) {
    if (queue == NULL || atomic_load(&queue->initialized) == 0) {
        return 1;
    }
    return atomic_load(&queue->head) == atomic_load(&queue->tail);
}

int lfq_is_full(lockfree_queue_t* queue) {
    if (queue == NULL || atomic_load(&queue->initialized) == 0) {
        return 1;
    }
    size_t head = atomic_load(&queue->head);
    size_t tail = atomic_load(&queue->tail);
    return ((tail + 1) % queue->capacity) == head;
}

size_t lfq_size(lockfree_queue_t* queue) {
    if (queue == NULL || atomic_load(&queue->initialized) == 0) {
        return 0;
    }
    size_t head = atomic_load(&queue->head);
    size_t tail = atomic_load(&queue->tail);
    if (tail >= head) {
        return tail - head;
    } else {
        return queue->capacity - head + tail;
    }
}
