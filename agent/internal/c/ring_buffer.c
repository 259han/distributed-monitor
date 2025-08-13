#include "ring_buffer.h"
#include <stdio.h>
#include <assert.h>

ring_buffer_t* ring_buffer_create(size_t initial_capacity, int blocking) {
    if (initial_capacity < MIN_CAPACITY) {
        initial_capacity = MIN_CAPACITY;
    }
    if (initial_capacity > MAX_CAPACITY) {
        initial_capacity = MAX_CAPACITY;
    }

    ring_buffer_t *rb = (ring_buffer_t*)malloc(sizeof(ring_buffer_t));
    if (!rb) {
        return NULL;
    }

    // 分配缓冲区
    rb->buffer = (void**)malloc(initial_capacity * sizeof(void*));
    if (!rb->buffer) {
        free(rb);
        return NULL;
    }

    // 初始化属性
    rb->capacity = initial_capacity;
    rb->size = 0;
    rb->head = 0;
    rb->tail = 0;
    rb->blocking = blocking;
    atomic_init(&rb->ref_count, 1);

    // 初始化锁和条件变量
    if (pthread_mutex_init(&rb->mutex, NULL) != 0) {
        free(rb->buffer);
        free(rb);
        return NULL;
    }

    if (pthread_cond_init(&rb->not_empty, NULL) != 0) {
        pthread_mutex_destroy(&rb->mutex);
        free(rb->buffer);
        free(rb);
        return NULL;
    }

    if (pthread_cond_init(&rb->not_full, NULL) != 0) {
        pthread_mutex_destroy(&rb->mutex);
        pthread_cond_destroy(&rb->not_empty);
        free(rb->buffer);
        free(rb);
        return NULL;
    }

    return rb;
}

void ring_buffer_destroy(ring_buffer_t *rb) {
    if (!rb) {
        return;
    }

    ring_buffer_unref(rb);
}

ring_buffer_t* ring_buffer_ref(ring_buffer_t *rb) {
    if (!rb) {
        return NULL;
    }

    atomic_fetch_add(&rb->ref_count, 1);
    return rb;
}

void ring_buffer_unref(ring_buffer_t *rb) {
    if (!rb) {
        return;
    }

    if (atomic_fetch_sub(&rb->ref_count, 1) == 1) {
        // 销毁资源
        pthread_mutex_destroy(&rb->mutex);
        pthread_cond_destroy(&rb->not_empty);
        pthread_cond_destroy(&rb->not_full);
        free(rb->buffer);
        free(rb);
    }
}

int ring_buffer_push(ring_buffer_t *rb, void *data) {
    if (!rb || !data) {
        return -1;
    }

    pthread_mutex_lock(&rb->mutex);

    // 如果已满且是阻塞模式，等待
    while (rb->size >= rb->capacity && rb->blocking) {
        pthread_cond_wait(&rb->not_full, &rb->mutex);
    }

    // 如果已满且不是阻塞模式，尝试扩展
    if (rb->size >= rb->capacity) {
        if (rb->capacity >= MAX_CAPACITY) {
            pthread_mutex_unlock(&rb->mutex);
            return -1; // 已达到最大容量
        }
        
        // 尝试扩展容量
        size_t new_capacity = rb->capacity * 2;
        if (new_capacity > MAX_CAPACITY) {
            new_capacity = MAX_CAPACITY;
        }
        
        if (ring_buffer_expand(rb, new_capacity) != 0) {
            pthread_mutex_unlock(&rb->mutex);
            return -1;
        }
    }

    // 添加数据
    rb->buffer[rb->tail] = data;
    rb->tail = (rb->tail + 1) % rb->capacity;
    rb->size++;

    // 通知等待的消费者
    pthread_cond_signal(&rb->not_empty);
    pthread_mutex_unlock(&rb->mutex);

    return 0;
}

int ring_buffer_pop(ring_buffer_t *rb, void **data) {
    if (!rb || !data) {
        return -1;
    }

    pthread_mutex_lock(&rb->mutex);

    // 如果为空且是阻塞模式，等待
    while (rb->size == 0 && rb->blocking) {
        pthread_cond_wait(&rb->not_empty, &rb->mutex);
    }

    // 如果为空且不是阻塞模式，返回错误
    if (rb->size == 0) {
        pthread_mutex_unlock(&rb->mutex);
        return -1;
    }

    // 获取数据
    *data = rb->buffer[rb->head];
    rb->head = (rb->head + 1) % rb->capacity;
    rb->size--;

    // 通知等待的生产者
    pthread_cond_signal(&rb->not_full);
    pthread_mutex_unlock(&rb->mutex);

    return 0;
}

int ring_buffer_try_pop(ring_buffer_t *rb, void **data) {
    if (!rb || !data) {
        return -1;
    }

    pthread_mutex_lock(&rb->mutex);

    if (rb->size == 0) {
        pthread_mutex_unlock(&rb->mutex);
        return -1;
    }

    // 获取数据
    *data = rb->buffer[rb->head];
    rb->head = (rb->head + 1) % rb->capacity;
    rb->size--;

    // 通知等待的生产者
    pthread_cond_signal(&rb->not_full);
    pthread_mutex_unlock(&rb->mutex);

    return 0;
}

int ring_buffer_push_batch(ring_buffer_t *rb, void **data, size_t count) {
    if (!rb || !data || count == 0) {
        return -1;
    }

    pthread_mutex_lock(&rb->mutex);

    // 计算可用空间
    size_t available = rb->capacity - rb->size;
    
    // 如果空间不足且是阻塞模式，等待
    while (available < count && rb->blocking) {
        pthread_cond_wait(&rb->not_full, &rb->mutex);
        available = rb->capacity - rb->size;
    }

    // 如果空间不足且不是阻塞模式，尝试扩展
    if (available < count) {
        size_t required_capacity = rb->size + count;
        if (required_capacity > MAX_CAPACITY) {
            pthread_mutex_unlock(&rb->mutex);
            return -1; // 超过最大容量
        }
        
        // 计算新的容量（2的幂次方）
        size_t new_capacity = rb->capacity;
        while (new_capacity < required_capacity) {
            new_capacity *= 2;
        }
        if (new_capacity > MAX_CAPACITY) {
            new_capacity = MAX_CAPACITY;
        }
        
        if (ring_buffer_expand(rb, new_capacity) != 0) {
            pthread_mutex_unlock(&rb->mutex);
            return -1;
        }
    }

    // 批量添加数据
    for (size_t i = 0; i < count; i++) {
        rb->buffer[rb->tail] = data[i];
        rb->tail = (rb->tail + 1) % rb->capacity;
    }
    rb->size += count;

    // 通知等待的消费者
    pthread_cond_broadcast(&rb->not_empty);
    pthread_mutex_unlock(&rb->mutex);

    return 0;
}

int ring_buffer_pop_batch(ring_buffer_t *rb, void **data, size_t max_count) {
    if (!rb || !data || max_count == 0) {
        return -1;
    }

    pthread_mutex_lock(&rb->mutex);

    // 计算可获取的数量
    size_t count = rb->size < max_count ? rb->size : max_count;
    
    if (count == 0) {
        pthread_mutex_unlock(&rb->mutex);
        return 0;
    }

    // 批量获取数据
    for (size_t i = 0; i < count; i++) {
        data[i] = rb->buffer[rb->head];
        rb->head = (rb->head + 1) % rb->capacity;
    }
    rb->size -= count;

    // 通知等待的生产者
    pthread_cond_broadcast(&rb->not_full);
    pthread_mutex_unlock(&rb->mutex);

    return count;
}

size_t ring_buffer_size(ring_buffer_t *rb) {
    if (!rb) {
        return 0;
    }

    pthread_mutex_lock(&rb->mutex);
    size_t size = rb->size;
    pthread_mutex_unlock(&rb->mutex);

    return size;
}

size_t ring_buffer_capacity(ring_buffer_t *rb) {
    if (!rb) {
        return 0;
    }

    pthread_mutex_lock(&rb->mutex);
    size_t capacity = rb->capacity;
    pthread_mutex_unlock(&rb->mutex);

    return capacity;
}

int ring_buffer_is_empty(ring_buffer_t *rb) {
    if (!rb) {
        return 1;
    }

    pthread_mutex_lock(&rb->mutex);
    int empty = (rb->size == 0);
    pthread_mutex_unlock(&rb->mutex);

    return empty;
}

int ring_buffer_is_full(ring_buffer_t *rb) {
    if (!rb) {
        return 1;
    }

    pthread_mutex_lock(&rb->mutex);
    int full = (rb->size >= rb->capacity);
    pthread_mutex_unlock(&rb->mutex);

    return full;
}

void ring_buffer_clear(ring_buffer_t *rb) {
    if (!rb) {
        return;
    }

    pthread_mutex_lock(&rb->mutex);
    rb->head = 0;
    rb->tail = 0;
    rb->size = 0;
    pthread_cond_broadcast(&rb->not_full);
    pthread_mutex_unlock(&rb->mutex);
}

int ring_buffer_expand(ring_buffer_t *rb, size_t new_capacity) {
    if (!rb || new_capacity <= rb->capacity) {
        return -1;
    }

    // 分配新的缓冲区
    void **new_buffer = (void**)malloc(new_capacity * sizeof(void*));
    if (!new_buffer) {
        return -1;
    }

    // 复制数据
    if (rb->size > 0) {
        if (rb->head < rb->tail) {
            // 数据在中间，直接复制
            memcpy(new_buffer, rb->buffer + rb->head, rb->size * sizeof(void*));
        } else {
            // 数据跨越边界，分两段复制
            size_t first_part = rb->capacity - rb->head;
            memcpy(new_buffer, rb->buffer + rb->head, first_part * sizeof(void*));
            memcpy(new_buffer + first_part, rb->buffer, rb->tail * sizeof(void*));
        }
    }

    // 更新属性
    free(rb->buffer);
    rb->buffer = new_buffer;
    rb->capacity = new_capacity;
    rb->head = 0;
    rb->tail = rb->size;

    return 0;
}

int ring_buffer_shrink(ring_buffer_t *rb, size_t new_capacity) {
    if (!rb || new_capacity < rb->size || new_capacity < MIN_CAPACITY) {
        return -1;
    }

    if (new_capacity >= rb->capacity) {
        return 0; // 不需要缩小
    }

    // 分配新的缓冲区
    void **new_buffer = (void**)malloc(new_capacity * sizeof(void*));
    if (!new_buffer) {
        return -1;
    }

    // 复制数据
    if (rb->size > 0) {
        if (rb->head < rb->tail) {
            // 数据在中间，直接复制
            memcpy(new_buffer, rb->buffer + rb->head, rb->size * sizeof(void*));
        } else {
            // 数据跨越边界，分两段复制
            size_t first_part = rb->capacity - rb->head;
            size_t copy_size = rb->size < first_part ? rb->size : first_part;
            memcpy(new_buffer, rb->buffer + rb->head, copy_size * sizeof(void*));
            if (rb->size > first_part) {
                memcpy(new_buffer + first_part, rb->buffer, (rb->size - first_part) * sizeof(void*));
            }
        }
    }

    // 更新属性
    free(rb->buffer);
    rb->buffer = new_buffer;
    rb->capacity = new_capacity;
    rb->head = 0;
    rb->tail = rb->size;

    return 0;
}

int ring_buffer_auto_resize(ring_buffer_t *rb) {
    if (!rb) {
        return -1;
    }

    pthread_mutex_lock(&rb->mutex);

    // 如果使用率超过80%，考虑扩展
    if (rb->size > rb->capacity * 0.8 && rb->capacity < MAX_CAPACITY) {
        size_t new_capacity = rb->capacity * 2;
        if (new_capacity > MAX_CAPACITY) {
            new_capacity = MAX_CAPACITY;
        }
        
        if (ring_buffer_expand(rb, new_capacity) != 0) {
            pthread_mutex_unlock(&rb->mutex);
            return -1;
        }
    }
    // 如果使用率低于25%，考虑缩小
    else if (rb->size < rb->capacity * 0.25 && rb->capacity > MIN_CAPACITY) {
        size_t new_capacity = rb->capacity / 2;
        if (new_capacity < MIN_CAPACITY) {
            new_capacity = MIN_CAPACITY;
        }
        
        if (ring_buffer_shrink(rb, new_capacity) != 0) {
            pthread_mutex_unlock(&rb->mutex);
            return -1;
        }
    }

    pthread_mutex_unlock(&rb->mutex);
    return 0;
}