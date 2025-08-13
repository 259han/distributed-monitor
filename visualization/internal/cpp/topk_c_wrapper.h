#ifndef TOPK_C_WRAPPER_H
#define TOPK_C_WRAPPER_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// C接口结构定义
typedef struct {
    char host_id[64];
    double value;
    int64_t timestamp;
} TopKItem_C;

typedef struct {
    TopKItem_C* items;
    int count;
} TopKResult_C;

typedef struct {
    int64_t total_items;
    int active_items;
    int expired_items;
    int64_t total_operations;
    int64_t uptime;
    int64_t memory_usage;
} TopKStats_C;

// C接口函数
typedef void* FastTopK_Handle;

// 创建FastTopK实例
FastTopK_Handle FastTopK_Create(int k, int ttl_seconds);

// 销毁FastTopK实例
void FastTopK_Destroy(FastTopK_Handle handle);

// 添加元素
int FastTopK_Add(FastTopK_Handle handle, const char* host_id, double value);

// 获取Top-K结果
TopKResult_C* FastTopK_GetTopK(FastTopK_Handle handle);

// 释放结果
void FastTopK_FreeResult(TopKResult_C* result);

// 获取当前元素数量
int FastTopK_GetCount(FastTopK_Handle handle);

// 清空所有元素
void FastTopK_Clear(FastTopK_Handle handle);

// 设置TTL
void FastTopK_SetTTL(FastTopK_Handle handle, int seconds);

// 获取TTL
int FastTopK_GetTTL(FastTopK_Handle handle);

// 获取K值
int FastTopK_GetK(FastTopK_Handle handle);

// 设置K值
int FastTopK_SetK(FastTopK_Handle handle, int k);

// 获取特定主机的值
double FastTopK_GetValue(FastTopK_Handle handle, const char* host_id);

// 移除特定主机
int FastTopK_Remove(FastTopK_Handle handle, const char* host_id);

// 检查是否包含特定主机
int FastTopK_Contains(FastTopK_Handle handle, const char* host_id);

// 获取过期项目数量
int FastTopK_GetExpiredCount(FastTopK_Handle handle);

// 清理过期项目
int FastTopK_CleanupExpired(FastTopK_Handle handle);

// 获取统计信息
TopKStats_C* FastTopK_GetStats(FastTopK_Handle handle);

// 释放统计信息
void FastTopK_FreeStats(TopKStats_C* stats);

// 获取过滤后的Top-K结果
TopKResult_C* FastTopK_GetTopKFiltered(FastTopK_Handle handle, double min_value, double max_value);

// 获取符合主机ID模式的Top-K结果
TopKResult_C* FastTopK_GetTopKByHostIDPattern(FastTopK_Handle handle, const char* pattern);

// 导出为JSON字符串
char* FastTopK_ExportToJSON(FastTopK_Handle handle);

// 从JSON字符串导入
int FastTopK_ImportFromJSON(FastTopK_Handle handle, const char* json_str);

// 启用调试模式
void FastTopK_EnableDebug(FastTopK_Handle handle, int enable);

#ifdef __cplusplus
}
#endif

#endif // TOPK_C_WRAPPER_H