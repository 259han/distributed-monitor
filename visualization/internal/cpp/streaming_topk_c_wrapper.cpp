#include "streaming_topk_c_wrapper.h"
#include "streaming_topk.h"
#include <cstring>
#include <memory>
#include <vector>
#include <string>

// StreamingTopKProcessor C包装器实现

extern "C" {

StreamingTopKProcessor_Handle StreamingTopKProcessor_Create(int k, int ttl_seconds) {
    try {
        auto processor = new StreamingTopKProcessor(k, ttl_seconds);
        return static_cast<StreamingTopKProcessor_Handle>(processor);
    } catch (...) {
        return nullptr;
    }
}

void StreamingTopKProcessor_Destroy(StreamingTopKProcessor_Handle handle) {
    if (handle) {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        delete processor;
    }
}

int StreamingTopKProcessor_ProcessStream(StreamingTopKProcessor_Handle handle, const char* host_id, double value) {
    if (!handle || !host_id) {
        return -1;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        processor->process_stream(std::string(host_id), value);
        return 0;
    } catch (...) {
        return -1;
    }
}

int StreamingTopKProcessor_ProcessStreamBatch(StreamingTopKProcessor_Handle handle, StreamingTopKItem_C* items, int count) {
    if (!handle || !items || count <= 0) {
        return -1;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        std::vector<std::pair<std::string, double>> batch_items;
        batch_items.reserve(count);
        
        for (int i = 0; i < count; i++) {
            batch_items.emplace_back(std::string(items[i].host_id), items[i].value);
        }
        
        processor->process_stream_batch(batch_items);
        return 0;
    } catch (...) {
        return -1;
    }
}

StreamingTopKResult_C* StreamingTopKProcessor_GetTopK(StreamingTopKProcessor_Handle handle) {
    if (!handle) {
        return nullptr;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        auto results = processor->get_top_k();
        
        auto result = new StreamingTopKResult_C;
        result->count = static_cast<int>(results.size());
        
        if (result->count > 0) {
            result->items = new StreamingTopKItem_C[result->count];
            for (int i = 0; i < result->count; i++) {
                const auto& item = results[i];
                strncpy(result->items[i].host_id, item.host_id.c_str(), sizeof(result->items[i].host_id) - 1);
                result->items[i].host_id[sizeof(result->items[i].host_id) - 1] = '\0';
                result->items[i].value = item.value;
                result->items[i].timestamp = item.timestamp;
                result->items[i].index = item.index;
                result->items[i].update_count = item.update_count;
            }
        } else {
            result->items = nullptr;
        }
        
        return result;
    } catch (...) {
        return nullptr;
    }
}

void StreamingTopKProcessor_FreeResult(StreamingTopKResult_C* result) {
    if (result) {
        if (result->items) {
            delete[] result->items;
        }
        delete result;
    }
}

StreamingTopKStats_C* StreamingTopKProcessor_GetStats(StreamingTopKProcessor_Handle handle) {
    if (!handle) {
        return nullptr;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        auto stats = processor->get_stats();
        
        auto result = new StreamingTopKStats_C;
        result->total_updates = std::any_cast<int64_t>(stats["total_updates"]);
        result->current_items = std::any_cast<int>(stats["current_items"]);
        result->k_value = std::any_cast<int>(stats["k_value"]);
        result->ttl_seconds = std::any_cast<int>(stats["ttl_seconds"]);
        result->is_closed = std::any_cast<bool>(stats["is_closed"]);
        
        return result;
    } catch (...) {
        return nullptr;
    }
}

void StreamingTopKProcessor_FreeStats(StreamingTopKStats_C* stats) {
    if (stats) {
        delete stats;
    }
}

int StreamingTopKProcessor_GetCount(StreamingTopKProcessor_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        return processor->get_count();
    } catch (...) {
        return 0;
    }
}

int StreamingTopKProcessor_Contains(StreamingTopKProcessor_Handle handle, const char* host_id) {
    if (!handle || !host_id) {
        return 0;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        return processor->contains(std::string(host_id)) ? 1 : 0;
    } catch (...) {
        return 0;
    }
}

int StreamingTopKProcessor_GetValue(StreamingTopKProcessor_Handle handle, const char* host_id, double* value) {
    if (!handle || !host_id || !value) {
        return 0;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        double val;
        bool found = processor->get_value(std::string(host_id), val);
        if (found) {
            *value = val;
            return 1;
        }
        return 0;
    } catch (...) {
        return 0;
    }
}

void StreamingTopKProcessor_Clear(StreamingTopKProcessor_Handle handle) {
    if (handle) {
        try {
            auto processor = static_cast<StreamingTopKProcessor*>(handle);
            processor->clear();
        } catch (...) {
            // 忽略异常
        }
    }
}

void StreamingTopKProcessor_SetTTL(StreamingTopKProcessor_Handle handle, int seconds) {
    if (handle) {
        try {
            auto processor = static_cast<StreamingTopKProcessor*>(handle);
            processor->set_ttl(seconds);
        } catch (...) {
            // 忽略异常
        }
    }
}

int StreamingTopKProcessor_GetTTL(StreamingTopKProcessor_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        return processor->get_ttl();
    } catch (...) {
        return 0;
    }
}

int StreamingTopKProcessor_GetK(StreamingTopKProcessor_Handle handle) {
    if (!handle) {
        return 0;
    }
    
    try {
        auto processor = static_cast<StreamingTopKProcessor*>(handle);
        return processor->get_k();
    } catch (...) {
        return 0;
    }
}

void StreamingTopKProcessor_SetK(StreamingTopKProcessor_Handle handle, int k) {
    if (handle) {
        try {
            auto processor = static_cast<StreamingTopKProcessor*>(handle);
            processor->set_k(k);
        } catch (...) {
            // 忽略异常
        }
    }
}

void StreamingTopKProcessor_Close(StreamingTopKProcessor_Handle handle) {
    if (handle) {
        try {
            auto processor = static_cast<StreamingTopKProcessor*>(handle);
            processor->close();
        } catch (...) {
            // 忽略异常
        }
    }
}

// StreamingTopKManager C包装器实现

StreamingTopKManager_Handle StreamingTopKManager_GetInstance() {
    try {
        auto& manager = StreamingTopKManager::get_instance();
        return static_cast<StreamingTopKManager_Handle>(&manager);
    } catch (...) {
        return nullptr;
    }
}

int StreamingTopKManager_Initialize(StreamingTopKManager_Handle handle, int default_k, int default_ttl) {
    if (!handle) {
        return -1;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        return manager->initialize(default_k, default_ttl) ? 0 : -1;
    } catch (...) {
        return -1;
    }
}

void StreamingTopKManager_Destroy(StreamingTopKManager_Handle handle) {
    // 单例模式，不需要销毁
    (void)handle;
}

StreamingTopKProcessor_Handle StreamingTopKManager_CreateProcessor(StreamingTopKManager_Handle handle, const char* name, int k, int ttl_seconds) {
    if (!handle || !name) {
        return nullptr;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        auto processor = manager->create_processor(std::string(name), k, ttl_seconds);
        if (processor) {
            return static_cast<StreamingTopKProcessor_Handle>(processor.get());
        }
        return nullptr;
    } catch (...) {
        return nullptr;
    }
}

StreamingTopKProcessor_Handle StreamingTopKManager_GetProcessor(StreamingTopKManager_Handle handle, const char* name) {
    if (!handle || !name) {
        return nullptr;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        auto processor = manager->get_processor(std::string(name));
        if (processor) {
            return static_cast<StreamingTopKProcessor_Handle>(processor.get());
        }
        return nullptr;
    } catch (...) {
        return nullptr;
    }
}

int StreamingTopKManager_RemoveProcessor(StreamingTopKManager_Handle handle, const char* name) {
    if (!handle || !name) {
        return 0;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        return manager->remove_processor(std::string(name)) ? 1 : 0;
    } catch (...) {
        return 0;
    }
}

int StreamingTopKManager_AddMetric(StreamingTopKManager_Handle handle, const char* processor_name, const char* host_id, double value) {
    if (!handle || !processor_name || !host_id) {
        return -1;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        manager->add_metric(std::string(processor_name), std::string(host_id), value);
        return 0;
    } catch (...) {
        return -1;
    }
}

StreamingTopKResult_C* StreamingTopKManager_GetTopK(StreamingTopKManager_Handle handle, const char* processor_name) {
    if (!handle || !processor_name) {
        return nullptr;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        auto results = manager->get_top_k(std::string(processor_name));
        
        auto result = new StreamingTopKResult_C;
        result->count = static_cast<int>(results.size());
        
        if (result->count > 0) {
            result->items = new StreamingTopKItem_C[result->count];
            for (int i = 0; i < result->count; i++) {
                const auto& item = results[i];
                strncpy(result->items[i].host_id, item.host_id.c_str(), sizeof(result->items[i].host_id) - 1);
                result->items[i].host_id[sizeof(result->items[i].host_id) - 1] = '\0';
                result->items[i].value = item.value;
                result->items[i].timestamp = item.timestamp;
                result->items[i].index = item.index;
                result->items[i].update_count = item.update_count;
            }
        } else {
            result->items = nullptr;
        }
        
        return result;
    } catch (...) {
        return nullptr;
    }
}

void StreamingTopKManager_FreeResult(StreamingTopKResult_C* result) {
    if (result) {
        if (result->items) {
            delete[] result->items;
        }
        delete result;
    }
}

StreamingTopKNames_C* StreamingTopKManager_GetProcessorNames(StreamingTopKManager_Handle handle) {
    if (!handle) {
        return nullptr;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        auto names = manager->get_processor_names();
        
        auto result = new StreamingTopKNames_C;
        result->count = static_cast<int>(names.size());
        
        if (result->count > 0) {
            result->names = new char*[result->count];
            for (int i = 0; i < result->count; i++) {
                const auto& name = names[i];
                result->names[i] = new char[name.length() + 1];
                strcpy(result->names[i], name.c_str());
            }
        } else {
            result->names = nullptr;
        }
        
        return result;
    } catch (...) {
        return nullptr;
    }
}

void StreamingTopKManager_FreeNames(StreamingTopKNames_C* names) {
    if (names) {
        if (names->names) {
            for (int i = 0; i < names->count; i++) {
                delete[] names->names[i];
            }
            delete[] names->names;
        }
        delete names;
    }
}

StreamingTopKSystemStats_C* StreamingTopKManager_GetSystemStats(StreamingTopKManager_Handle handle) {
    if (!handle) {
        return nullptr;
    }
    
    try {
        auto manager = static_cast<StreamingTopKManager*>(handle);
        auto stats = manager->get_system_stats();
        
        auto result = new StreamingTopKSystemStats_C;
        result->total_processors = std::any_cast<int>(stats["total_processors"]);
        result->initialized = std::any_cast<bool>(stats["initialized"]) ? 1 : 0;
        result->default_k = std::any_cast<int>(stats["default_k"]);
        result->default_ttl = std::any_cast<int>(stats["default_ttl"]);
        result->running = std::any_cast<bool>(stats["running"]) ? 1 : 0;
        result->total_updates = std::any_cast<int64_t>(stats["total_updates"]);
        result->total_items = std::any_cast<int>(stats["total_items"]);
        
        return result;
    } catch (...) {
        return nullptr;
    }
}

void StreamingTopKManager_FreeStats(StreamingTopKSystemStats_C* stats) {
    if (stats) {
        delete stats;
    }
}

void StreamingTopKManager_Shutdown(StreamingTopKManager_Handle handle) {
    if (handle) {
        try {
            auto manager = static_cast<StreamingTopKManager*>(handle);
            manager->shutdown();
        } catch (...) {
            // 忽略异常
        }
    }
}

} // extern "C"
