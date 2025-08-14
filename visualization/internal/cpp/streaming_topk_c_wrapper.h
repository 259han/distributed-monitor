#ifndef STREAMING_TOP_K_C_WRAPPER_H
#define STREAMING_TOP_K_C_WRAPPER_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

// C结构体定义
typedef struct {
    char host_id[64];
    double value;
    int64_t timestamp;
    int32_t index;
    int32_t update_count;
} StreamingTopKItem_C;

typedef struct {
    StreamingTopKItem_C* items;
    int count;
} StreamingTopKResult_C;

typedef struct {
    char** names;
    int count;
} StreamingTopKNames_C;

typedef struct {
    int64_t total_updates;
    int current_items;
    int k_value;
    int ttl_seconds;
    int is_closed;
} StreamingTopKStats_C;

typedef struct {
    int total_processors;
    int initialized;
    int default_k;
    int default_ttl;
    int running;
    int64_t total_updates;
    int total_items;
} StreamingTopKSystemStats_C;

// 处理器句柄
typedef void* StreamingTopKProcessor_Handle;
typedef void* StreamingTopKManager_Handle;

// StreamingTopKProcessor C接口
StreamingTopKProcessor_Handle StreamingTopKProcessor_Create(int k, int ttl_seconds);
void StreamingTopKProcessor_Destroy(StreamingTopKProcessor_Handle handle);

int StreamingTopKProcessor_ProcessStream(StreamingTopKProcessor_Handle handle, const char* host_id, double value);
int StreamingTopKProcessor_ProcessStreamBatch(StreamingTopKProcessor_Handle handle, StreamingTopKItem_C* items, int count);

StreamingTopKResult_C* StreamingTopKProcessor_GetTopK(StreamingTopKProcessor_Handle handle);
void StreamingTopKProcessor_FreeResult(StreamingTopKResult_C* result);

StreamingTopKStats_C* StreamingTopKProcessor_GetStats(StreamingTopKProcessor_Handle handle);
void StreamingTopKProcessor_FreeStats(StreamingTopKStats_C* stats);

int StreamingTopKProcessor_GetCount(StreamingTopKProcessor_Handle handle);
int StreamingTopKProcessor_Contains(StreamingTopKProcessor_Handle handle, const char* host_id);
int StreamingTopKProcessor_GetValue(StreamingTopKProcessor_Handle handle, const char* host_id, double* value);

void StreamingTopKProcessor_Clear(StreamingTopKProcessor_Handle handle);
void StreamingTopKProcessor_SetTTL(StreamingTopKProcessor_Handle handle, int seconds);
int StreamingTopKProcessor_GetTTL(StreamingTopKProcessor_Handle handle);
int StreamingTopKProcessor_GetK(StreamingTopKProcessor_Handle handle);
void StreamingTopKProcessor_SetK(StreamingTopKProcessor_Handle handle, int k);
void StreamingTopKProcessor_Close(StreamingTopKProcessor_Handle handle);

// StreamingTopKManager C接口
StreamingTopKManager_Handle StreamingTopKManager_GetInstance();
int StreamingTopKManager_Initialize(StreamingTopKManager_Handle handle, int default_k, int default_ttl);
void StreamingTopKManager_Destroy(StreamingTopKManager_Handle handle);

StreamingTopKProcessor_Handle StreamingTopKManager_CreateProcessor(StreamingTopKManager_Handle handle, const char* name, int k, int ttl_seconds);
StreamingTopKProcessor_Handle StreamingTopKManager_GetProcessor(StreamingTopKManager_Handle handle, const char* name);
int StreamingTopKManager_RemoveProcessor(StreamingTopKManager_Handle handle, const char* name);

int StreamingTopKManager_AddMetric(StreamingTopKManager_Handle handle, const char* processor_name, const char* host_id, double value);
StreamingTopKResult_C* StreamingTopKManager_GetTopK(StreamingTopKManager_Handle handle, const char* processor_name);
void StreamingTopKManager_FreeResult(StreamingTopKResult_C* result);

StreamingTopKNames_C* StreamingTopKManager_GetProcessorNames(StreamingTopKManager_Handle handle);
void StreamingTopKManager_FreeNames(StreamingTopKNames_C* names);

StreamingTopKSystemStats_C* StreamingTopKManager_GetSystemStats(StreamingTopKManager_Handle handle);
void StreamingTopKManager_FreeStats(StreamingTopKSystemStats_C* stats);

void StreamingTopKManager_Shutdown(StreamingTopKManager_Handle handle);

#ifdef __cplusplus
}
#endif

#endif // STREAMING_TOP_K_C_WRAPPER_H
