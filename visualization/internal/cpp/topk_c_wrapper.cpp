#include "topk_c_wrapper.h"
#include "topk_shared.h"
#include <stdlib.h>
#include <string.h>

// C接口实现
extern "C" {

FastTopK_Handle FastTopK_Create(int k, int ttl_seconds) {
    FastTopK* topk = new FastTopK(k, ttl_seconds);
    return (FastTopK_Handle)topk;
}

void FastTopK_Destroy(FastTopK_Handle handle) {
    if (handle) {
        FastTopK* topk = (FastTopK*)handle;
        delete topk;
    }
}

int FastTopK_Add(FastTopK_Handle handle, const char* host_id, double value) {
    if (!handle || !host_id) {
        return -1;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    topk->add(host_id, value);
    return 0;
}

TopKResult_C* FastTopK_GetTopK(FastTopK_Handle handle) {
    if (!handle) {
        return nullptr;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    auto results = topk->get_top_k();
    
    if (results.empty()) {
        return nullptr;
    }
    
    // 分配结果结构
    TopKResult_C* result = (TopKResult_C*)malloc(sizeof(TopKResult_C));
    if (!result) {
        return nullptr;
    }
    
    // 分配项数组
    result->items = (TopKItem_C*)malloc(results.size() * sizeof(TopKItem_C));
    if (!result->items) {
        free(result);
        return nullptr;
    }
    
    // 填充数据
    result->count = results.size();
    for (size_t i = 0; i < results.size(); i++) {
        strncpy(result->items[i].host_id, results[i].host_id.c_str(), sizeof(result->items[i].host_id) - 1);
        result->items[i].host_id[sizeof(result->items[i].host_id) - 1] = '\0';
        result->items[i].value = results[i].value;
        result->items[i].timestamp = results[i].timestamp;
    }
    
    return result;
}

void FastTopK_FreeResult(TopKResult_C* result) {
    if (result) {
        if (result->items) {
            free(result->items);
        }
        free(result);
    }
}

int FastTopK_GetCount(FastTopK_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    return topk->get_count();
}

void FastTopK_Clear(FastTopK_Handle handle) {
    if (handle) {
        FastTopK* topk = (FastTopK*)handle;
        topk->clear();
    }
}

void FastTopK_SetTTL(FastTopK_Handle handle, int seconds) {
    if (handle) {
        FastTopK* topk = (FastTopK*)handle;
        topk->set_ttl(seconds);
    }
}

int FastTopK_GetTTL(FastTopK_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    return topk->get_ttl();
}

int FastTopK_GetK(FastTopK_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    return topk->get_k();
}

int FastTopK_SetK(FastTopK_Handle handle, int k) {
    if (!handle || k <= 0) {
        return -1;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    return topk->set_k(k) ? 0 : -1;
}

double FastTopK_GetValue(FastTopK_Handle handle, const char* host_id) {
    if (!handle || !host_id) {
        return -1.0;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    double value;
    return topk->get_value(host_id, value) ? value : -1.0;
}

int FastTopK_Remove(FastTopK_Handle handle, const char* host_id) {
    if (!handle || !host_id) {
        return -1;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    return topk->remove(host_id) ? 0 : -1;
}

int FastTopK_Contains(FastTopK_Handle handle, const char* host_id) {
    if (!handle || !host_id) {
        return 0;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    return topk->contains(host_id) ? 1 : 0;
}

int FastTopK_GetExpiredCount(FastTopK_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    return topk->get_expired_count();
}

int FastTopK_CleanupExpired(FastTopK_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    topk->cleanup_expired();
    return 0;
}

TopKStats_C* FastTopK_GetStats(FastTopK_Handle handle) {
    if (!handle) {
        return nullptr;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    
    TopKStats_C* cStats = (TopKStats_C*)malloc(sizeof(TopKStats_C));
    if (!cStats) {
        return nullptr;
    }
    
    // 调用我们实现的get_stats方法
    int64_t total_items, total_operations, uptime, memory_usage;
    int active_items, expired_items;
    topk->get_stats(total_items, active_items, expired_items, total_operations, uptime, memory_usage);
    
    cStats->total_items = total_items;
    cStats->active_items = active_items;
    cStats->expired_items = expired_items;
    cStats->total_operations = total_operations;
    cStats->uptime = uptime;
    cStats->memory_usage = memory_usage;
    
    return cStats;
}

void FastTopK_FreeStats(TopKStats_C* stats) {
    if (stats) {
        free(stats);
    }
}

TopKResult_C* FastTopK_GetTopKFiltered(FastTopK_Handle handle, double min_value, double max_value) {
    if (!handle) {
        return nullptr;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    auto filter = [min_value, max_value](const TopKItem& item) {
        return item.value >= min_value && item.value <= max_value;
    };
    
    auto results = topk->get_top_k_filtered(filter);
    
    if (results.empty()) {
        return nullptr;
    }
    
    // 分配结果结构
    TopKResult_C* result = (TopKResult_C*)malloc(sizeof(TopKResult_C));
    if (!result) {
        return nullptr;
    }
    
    // 分配项数组
    result->items = (TopKItem_C*)malloc(results.size() * sizeof(TopKItem_C));
    if (!result->items) {
        free(result);
        return nullptr;
    }
    
    // 填充数据
    result->count = results.size();
    for (size_t i = 0; i < results.size(); i++) {
        strncpy(result->items[i].host_id, results[i].host_id.c_str(), sizeof(result->items[i].host_id) - 1);
        result->items[i].host_id[sizeof(result->items[i].host_id) - 1] = '\0';
        result->items[i].value = results[i].value;
        result->items[i].timestamp = results[i].timestamp;
    }
    
    return result;
}

TopKResult_C* FastTopK_GetTopKByHostIDPattern(FastTopK_Handle handle, const char* pattern) {
    if (!handle || !pattern) {
        return nullptr;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    auto results = topk->get_top_k(); // 简化实现，实际应该支持模式匹配
    
    if (results.empty()) {
        return nullptr;
    }
    
    // 分配结果结构
    TopKResult_C* result = (TopKResult_C*)malloc(sizeof(TopKResult_C));
    if (!result) {
        return nullptr;
    }
    
    // 分配项数组
    result->items = (TopKItem_C*)malloc(results.size() * sizeof(TopKItem_C));
    if (!result->items) {
        free(result);
        return nullptr;
    }
    
    // 填充数据
    result->count = results.size();
    for (size_t i = 0; i < results.size(); i++) {
        strncpy(result->items[i].host_id, results[i].host_id.c_str(), sizeof(result->items[i].host_id) - 1);
        result->items[i].host_id[sizeof(result->items[i].host_id) - 1] = '\0';
        result->items[i].value = results[i].value;
        result->items[i].timestamp = results[i].timestamp;
    }
    
    return result;
}

char* FastTopK_ExportToJSON(FastTopK_Handle handle) {
    if (!handle) {
        return nullptr;
    }
    
    FastTopK* topk = (FastTopK*)handle;
    std::string json = topk->to_json();
    
    if (json.empty()) {
        return nullptr;
    }
    
    char* result = (char*)malloc(json.length() + 1);
    if (!result) {
        return nullptr;
    }
    
    strcpy(result, json.c_str());
    return result;
}

int FastTopK_ImportFromJSON(FastTopK_Handle handle, const char* json_str) {
    if (!handle || !json_str) {
        return -1;
    }
    
    // 简化实现，实际应该解析JSON
    return 0;
}

void FastTopK_EnableDebug(FastTopK_Handle handle, int enable) {
    if (handle) {
        FastTopK* topk = (FastTopK*)handle;
        topk->enable_debug(enable == 1);
    }
}

} // extern "C"