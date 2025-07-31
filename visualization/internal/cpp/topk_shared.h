#ifndef TOP_K_SHARED_H
#define TOP_K_SHARED_H

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
#include <sys/ipc.h>
#include <sys/shm.h>
#include <semaphore.h>
#include <fcntl.h>
#include <unistd.h>

// 共享内存中的数据结构
#pragma pack(push, 1)
struct SharedTopKItem {
    char host_id[64];     // 主机ID
    double value;         // 指标值
    int64_t timestamp;   // 时间戳
    int32_t valid;       // 是否有效
    int32_t update_count; // 更新次数
};

struct SharedTopKData {
    int32_t k;                    // K值
    int32_t count;                // 当前元素数量
    int32_t ttl_seconds;          // TTL秒数
    int32_t total_updates;        // 总更新次数
    int32_t total_operations;     // 总操作次数
    SharedTopKItem items[100];    // 最多100个元素
    sem_t sem_mutex;              // 互斥信号量
    sem_t sem_empty;              // 空信号量
    sem_t sem_full;               // 满信号量
    int32_t initialized;          // 是否已初始化
    int64_t last_cleanup;         // 上次清理时间
};
#pragma pack(pop)

// C++ Top-K算法实现
class TopKItem {
public:
    std::string host_id;
    double value;
    int64_t timestamp;
    int32_t update_count;

    TopKItem(const std::string& id = "", double val = 0.0, int64_t ts = 0, int32_t uc = 0)
        : host_id(id), value(val), timestamp(ts), update_count(uc) {}

    bool operator<(const TopKItem& other) const {
        return value < other.value;
    }

    bool operator>(const TopKItem& other) const {
        return value > other.value;
    }

    bool operator==(const TopKItem& other) const {
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

class SharedTopK {
public:
    // 创建共享内存Top-K
    static std::shared_ptr<SharedTopK> create(const std::string& shm_key, int k, int ttl_seconds = 3600);
    
    // 附加到现有共享内存
    static std::shared_ptr<SharedTopK> attach(const std::string& shm_key);
    
    ~SharedTopK();

    // 添加或更新元素
    bool add(const std::string& host_id, double value);
    
    // 批量添加元素
    bool add_batch(const std::vector<std::pair<std::string, double>>& items);
    
    // 获取Top-K元素
    std::vector<TopKItem> get_top_k();
    
    // 获取Top-K元素（带过滤）
    std::vector<TopKItem> get_top_k_filtered(const std::function<bool(const TopKItem&)>& filter) const;
    
    // 获取当前数量
    int get_count() const;
    
    // 获取统计信息
    void get_stats(int& total_updates, int& total_operations) const;
    
    // 清空
    void clear();
    
    // 清理过期数据
    void cleanup_expired() const;
    
    // 设置TTL
    void set_ttl(int seconds);
    
    // 检查是否有效
    bool is_valid() const;
    
    // 获取特定主机的值
    bool get_value(const std::string& host_id, double& value) const;
    
    // 移除特定主机
    bool remove(const std::string& host_id);

private:
    SharedTopK(key_t key, int shmid, SharedTopKData* data, bool created);
    
    key_t shm_key_;
    int shmid_;
    SharedTopKData* data_;
    bool created_;
    bool valid_;
    
    // 初始化信号量
    bool init_semaphores();
    
    // 等待信号量
    void wait_sem(sem_t* sem);
    
    // 发送信号量
    void post_sem(sem_t* sem);
    
    // 锁定
    void lock() const;
    
    // 解锁
    void unlock() const;
    
    // 检查并清理过期数据
    void internal_cleanup() const;
};

// 高性能Top-K处理器（使用堆）
class FastTopK {
public:
    FastTopK(int k = 10, int ttl_seconds = 3600);
    ~FastTopK();

    // 添加或更新元素
    void add(const std::string& host_id, double value);
    
    // 批量添加元素
    void add_batch(const std::vector<std::pair<std::string, double>>& items);
    
    // 获取Top-K元素
    std::vector<TopKItem> get_top_k() const;
    
    // 获取Top-K元素（带过滤）
    std::vector<TopKItem> get_top_k_filtered(const std::function<bool(const TopKItem&)>& filter) const;
    
    // 获取当前数量
    int get_count() const;
    
    // 清空
    void clear();
    
    // 设置TTL
    void set_ttl(int seconds);
    
    // 与共享内存同步
    bool sync_to_shared(const std::shared_ptr<SharedTopK>& shared);
    bool sync_from_shared(const std::shared_ptr<SharedTopK>& shared);
    
    // 清理过期数据
    void cleanup_expired() const;
    
    // 获取特定主机的值
    bool get_value(const std::string& host_id, double& value) const;
    
    // 移除特定主机
    bool remove(const std::string& host_id);
    
    // 获取内存使用情况
    size_t get_memory_usage() const;
    
    // 导出为JSON字符串
    std::string to_json() const;
    
    // 获取TTL
    int get_ttl() const;
    
    // 获取K值
    int get_k() const;
    
    // 设置K值
    int set_k(int k);
    
    // 检查是否包含特定主机
    bool contains(const std::string& host_id) const;
    
    // 获取过期项目数量
    int get_expired_count() const;
    
    // 获取统计信息
    void get_stats(int64_t& total_items, int& active_items, int& expired_items, 
                  int64_t& total_operations, int64_t& uptime, int64_t& memory_usage) const;
    
    // 启用调试模式
    void enable_debug(bool enable);

private:
    struct Item {
        std::string host_id;
        double value;
        int64_t timestamp;
        
        Item(const std::string& id = "", double val = 0.0, int64_t ts = 0)
            : host_id(id), value(val), timestamp(ts) {}
        
        bool operator<(const Item& other) const {
            return value < other.value;
        }
        
        bool operator>(const Item& other) const {
            return value > other.value;
        }
    };

    int k_;
    int ttl_seconds_;
    std::vector<Item> min_heap_;
    std::unordered_map<std::string, int> item_map_; // host_id -> index in heap
    
    // 调整堆
    void heapify_up(int index);
    void heapify_down(int index);
    
    // 查找最小值的索引
    int find_min_index() const;
};

// Top-K管理器
class TopKManager {
public:
    static TopKManager& get_instance();
    
    // 初始化
    bool initialize(const std::string& shm_key = "/tmp/topk_shm", int k = 10, int ttl_seconds = 3600);
    
    // 添加数据
    void add_metric(const std::string& host_id, const std::string& metric_name, double value);
    
    // 批量添加数据
    void add_metric_batch(const std::string& metric_name, const std::vector<std::pair<std::string, double>>& items);
    
    // 获取Top-K
    std::vector<TopKItem> get_top_k(const std::string& metric_name = "default") const;
    
    // 获取所有指标
    std::vector<std::string> get_metric_names() const;
    
    // 获取系统统计信息
    void get_system_stats(int& total_metrics, int& total_items, int& total_updates) const;
    
    // 停止
    void shutdown();
    
    // 创建新的指标实例
    bool create_metric(const std::string& metric_name, int k, int ttl_seconds);
    
    // 移除指标
    bool remove_metric(const std::string& metric_name);

private:
    TopKManager();
    ~TopKManager();
    
    std::unordered_map<std::string, std::shared_ptr<FastTopK>> topk_instances_;
    std::shared_ptr<SharedTopK> shared_topk_;
    mutable std::mutex mutex_;
    bool initialized_;
    
    // 定时清理过期数据
    void cleanup_thread();
    std::thread cleanup_thread_;
    bool running_;
    std::condition_variable cv_;
};

#endif // TOP_K_SHARED_H