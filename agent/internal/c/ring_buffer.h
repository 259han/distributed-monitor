#ifndef RING_BUFFER_H
#define RING_BUFFER_H

#include <stdlib.h>
#include <string.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>

// 最小容量
#define MIN_CAPACITY 1024
// 最大容量
#define MAX_CAPACITY 4096

typedef struct {
    void **buffer;           // 数据指针数组
    size_t capacity;         // 当前容量
    size_t size;            // 当前大小
    size_t head;            // 头部索引
    size_t tail;            // 尾部索引
    pthread_mutex_t mutex;  // 互斥锁
    pthread_cond_t not_empty; // 非空条件变量
    pthread_cond_t not_full;  // 非满条件变量
    int blocking;           // 是否阻塞模式
    atomic_int ref_count;   // 引用计数
} ring_buffer_t;

// 创建ring buffer
ring_buffer_t* ring_buffer_create(size_t initial_capacity, int blocking);

// 销毁ring buffer
void ring_buffer_destroy(ring_buffer_t *rb);

// 获取ring buffer引用
ring_buffer_t* ring_buffer_ref(ring_buffer_t *rb);

// 释放ring buffer引用
void ring_buffer_unref(ring_buffer_t *rb);

// 向ring buffer添加数据
int ring_buffer_push(ring_buffer_t *rb, void *data);

// 从ring buffer获取数据
int ring_buffer_pop(ring_buffer_t *rb, void **data);

// 非阻塞尝试获取数据
int ring_buffer_try_pop(ring_buffer_t *rb, void **data);

// 批量添加数据
int ring_buffer_push_batch(ring_buffer_t *rb, void **data, size_t count);

// 批量获取数据
int ring_buffer_pop_batch(ring_buffer_t *rb, void **data, size_t max_count);

// 获取当前大小
size_t ring_buffer_size(ring_buffer_t *rb);

// 获取当前容量
size_t ring_buffer_capacity(ring_buffer_t *rb);

// 检查是否为空
int ring_buffer_is_empty(ring_buffer_t *rb);

// 检查是否已满
int ring_buffer_is_full(ring_buffer_t *rb);

// 清空ring buffer
void ring_buffer_clear(ring_buffer_t *rb);

// 扩展ring buffer容量
int ring_buffer_expand(ring_buffer_t *rb, size_t new_capacity);

// 缩小ring buffer容量
int ring_buffer_shrink(ring_buffer_t *rb, size_t new_capacity);

// 自动调整容量（根据使用情况）
int ring_buffer_auto_resize(ring_buffer_t *rb);

// Go MetricsData结构的C定义
#pragma pack(push, 1)
typedef struct {
    char name[64];        // 指标名称
    double value;         // 指标值
    char unit[32];        // 单位
} Metric;

typedef struct {
    char host_id[64];     // 主机ID
    int64_t timestamp;    // 时间戳
    Metric *metrics;      // 指标数组
    int metrics_count;    // 指标数量
} MetricsData;
#pragma pack(pop)

// 内存大小常量
#define sizeof_Metric sizeof(Metric)
#define sizeof_MetricsData sizeof(MetricsData)

#endif // RING_BUFFER_H