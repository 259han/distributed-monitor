#ifndef STREAMING_TOP_K_H
#define STREAMING_TOP_K_H

#include <string>
#include <vector>
#include <memory>
#include <chrono>
#include <functional>
#include <unordered_map>
#include <mutex>
#include <thread>
#include <condition_variable>
#include <algorithm>
#include <iostream>
#include <atomic>
#include <queue>
#include <any>

// 流式Top-K项
struct StreamingTopKItem {
    std::string host_id;
    double value;
    int64_t timestamp;
    int32_t index;  // 堆索引
    int32_t update_count;

    StreamingTopKItem(const std::string& id = "", double val = 0.0, int64_t ts = 0, int32_t idx = -1, int32_t uc = 0)
        : host_id(id), value(val), timestamp(ts), index(idx), update_count(uc) {}

    bool operator<(const StreamingTopKItem& other) const {
        return value < other.value;
    }

    bool operator>(const StreamingTopKItem& other) const {
        return value > other.value;
    }

    bool operator==(const StreamingTopKItem& other) const {
        return host_id == other.host_id;
    }

    // 计算与当前时间的时间差（秒）
    int64_t age() const {
        auto now = std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::system_clock::now().time_since_epoch()).count();
        return now - timestamp;
    }

    // 检查是否过期
    bool is_expired(int ttl_seconds) const {
        return age() > ttl_seconds;
    }
};

// 流式Top-K处理器
class StreamingTopKProcessor {
public:
    // 更新回调函数类型
    using UpdateCallback = std::function<void(const std::vector<StreamingTopKItem>&)>;
    // 过期回调函数类型
    using ExpireCallback = std::function<void(const std::string&)>;

    StreamingTopKProcessor(int k = 10, int ttl_seconds = 3600);
    ~StreamingTopKProcessor();

    // 设置回调函数
    void set_update_callback(UpdateCallback callback);
    void set_expire_callback(ExpireCallback callback);

    // 流式处理单个数据
    void process_stream(const std::string& host_id, double value);
    
    // 批量流式处理
    void process_stream_batch(const std::vector<std::pair<std::string, double>>& items);
    
    // 获取当前Top-K结果
    std::vector<StreamingTopKItem> get_top_k() const;
    
    // 获取带过滤的Top-K结果
    std::vector<StreamingTopKItem> get_top_k_filtered(const std::function<bool(const StreamingTopKItem&)>& filter) const;
    
    // 获取统计信息
    std::unordered_map<std::string, std::any> get_stats() const;
    
    // 获取当前元素数量
    int get_count() const;
    
    // 检查是否包含指定主机
    bool contains(const std::string& host_id) const;
    
    // 获取指定主机的值
    bool get_value(const std::string& host_id, double& value) const;
    
    // 清空所有数据
    void clear();
    
    // 设置TTL
    void set_ttl(int seconds);
    
    // 获取TTL
    int get_ttl() const;
    
    // 获取K值
    int get_k() const;
    
    // 设置K值
    void set_k(int k);
    
    // 关闭处理器
    void close();
    
    // 检查是否已关闭
    bool is_closed() const;

private:
    struct Item {
        std::string host_id;
        double value;
        int64_t timestamp;
        int32_t index;
        int32_t update_count;
        
        Item(const std::string& id = "", double val = 0.0, int64_t ts = 0, int32_t idx = -1, int32_t uc = 0)
            : host_id(id), value(val), timestamp(ts), index(idx), update_count(uc) {}
        
        bool operator<(const Item& other) const {
            return value < other.value;
        }
        
        bool operator>(const Item& other) const {
            return value > other.value;
        }
    };

    mutable std::mutex mutex_;
    std::vector<Item> min_heap_;
    std::unordered_map<std::string, int> item_map_; // host_id -> index in heap
    
    int k_;
    int ttl_seconds_;
    std::atomic<bool> closed_;
    std::atomic<int64_t> update_count_;
    std::chrono::system_clock::time_point last_update_;
    
    // 回调函数
    UpdateCallback update_callback_;
    ExpireCallback expire_callback_;
    
    // 调整堆
    void heapify_up(int index);
    void heapify_down(int index);
    
    // 清理过期数据
    void cleanup_expired(const std::chrono::system_clock::time_point& now);
    
    // 获取Top-K结果（内部方法，需要锁保护）
    std::vector<StreamingTopKItem> get_top_k_internal() const;
    
    // 触发更新回调
    void trigger_update_callback();
    
    // 触发过期回调
    void trigger_expire_callback(const std::string& host_id);
};

// 高性能流式Top-K管理器
class StreamingTopKManager {
public:
    // 单例模式
    static StreamingTopKManager& get_instance();
    
    // 初始化
    bool initialize(int default_k = 10, int default_ttl = 3600);
    
    // 创建新的Top-K实例
    std::shared_ptr<StreamingTopKProcessor> create_processor(const std::string& name, int k, int ttl_seconds);
    
    // 获取Top-K实例
    std::shared_ptr<StreamingTopKProcessor> get_processor(const std::string& name);
    
    // 移除Top-K实例
    bool remove_processor(const std::string& name);
    
    // 添加指标数据
    void add_metric(const std::string& processor_name, const std::string& host_id, double value);
    
    // 批量添加指标数据
    void add_metric_batch(const std::string& processor_name, const std::vector<std::pair<std::string, double>>& items);
    
    // 获取Top-K结果
    std::vector<StreamingTopKItem> get_top_k(const std::string& processor_name) const;
    
    // 获取所有处理器名称
    std::vector<std::string> get_processor_names() const;
    
    // 获取系统统计信息
    std::unordered_map<std::string, std::any> get_system_stats() const;
    
    // 停止管理器
    void shutdown();
    
    // 设置全局更新回调
    void set_global_update_callback(StreamingTopKProcessor::UpdateCallback callback);
    
    // 设置全局过期回调
    void set_global_expire_callback(StreamingTopKProcessor::ExpireCallback callback);

private:
    StreamingTopKManager();
    ~StreamingTopKManager();
    
    std::unordered_map<std::string, std::shared_ptr<StreamingTopKProcessor>> processors_;
    mutable std::mutex mutex_;
    bool initialized_;
    int default_k_;
    int default_ttl_;
    
    // 全局回调
    StreamingTopKProcessor::UpdateCallback global_update_callback_;
    StreamingTopKProcessor::ExpireCallback global_expire_callback_;
    
    // 定时清理线程
    void cleanup_thread();
    std::thread cleanup_thread_;
    std::atomic<bool> running_;
    std::condition_variable cv_;
};

// 无锁流式Top-K处理器
class LockFreeStreamingTopK {
public:
    LockFreeStreamingTopK(int k = 10, int ttl_seconds = 3600);
    ~LockFreeStreamingTopK();

    // 无锁流式处理
    void process_stream_lock_free(const std::string& host_id, double value);
    
    // 获取Top-K结果（需要锁保护）
    std::vector<StreamingTopKItem> get_top_k() const;
    
    // 获取统计信息
    std::unordered_map<std::string, std::any> get_stats() const;

private:
    struct LockFreeItem {
        std::string host_id;
        std::atomic<double> value;
        std::atomic<int64_t> timestamp;
        std::atomic<int32_t> index;
        std::atomic<int32_t> update_count;
        
        LockFreeItem() : value(0.0), timestamp(0), index(-1), update_count(0) {}
    };

    std::vector<std::unique_ptr<LockFreeItem>> items_;
    std::atomic<int> current_size_;
    int k_;
    int ttl_seconds_;
    std::atomic<int64_t> update_count_;
    
    // 无锁操作
    bool try_add_item(const std::string& host_id, double value);
    bool try_update_item(int index, double value);
    int find_min_index() const;
};

#endif // STREAMING_TOP_K_H
