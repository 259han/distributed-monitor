#include "topk_shared.h"
#include <algorithm>
#include <iostream>
#include <sys/stat.h>
#include <sys/types.h>
#include <cstring>
#include <thread>
#include <mutex>
#include <condition_variable>
#include <unordered_map>
#include <chrono>

// SharedTopK 实现
std::shared_ptr<SharedTopK> SharedTopK::create(const std::string& shm_key, int k, int ttl_seconds) {
    key_t key = ftok(shm_key.c_str(), 'R');
    if (key == -1) {
        return nullptr;
    }

    // 计算共享内存大小
    size_t shm_size = sizeof(SharedTopKData);
    
    // 创建共享内存
    int shmid = shmget(key, shm_size, IPC_CREAT | IPC_EXCL | 0666);
    if (shmid == -1) {
        if (errno == EEXIST) {
            // 共享内存已存在，尝试附加
            return attach(shm_key);
        }
        return nullptr;
    }

    // 附加共享内存
    SharedTopKData* data = (SharedTopKData*)shmat(shmid, nullptr, 0);
    if (data == (void*)-1) {
        shmctl(shmid, IPC_RMID, nullptr);
        return nullptr;
    }

    // 初始化数据
    memset(data, 0, sizeof(SharedTopKData));
    data->k = k;
    data->count = 0;
    data->ttl_seconds = ttl_seconds;
    data->initialized = 0;

    auto shared = std::shared_ptr<SharedTopK>(new SharedTopK(key, shmid, data, true));
    if (shared->init_semaphores()) {
        data->initialized = 1;
        return shared;
    }

    return nullptr;
}

std::shared_ptr<SharedTopK> SharedTopK::attach(const std::string& shm_key) {
    key_t key = ftok(shm_key.c_str(), 'R');
    if (key == -1) {
        return nullptr;
    }

    // 获取共享内存
    int shmid = shmget(key, sizeof(SharedTopKData), 0666);
    if (shmid == -1) {
        return nullptr;
    }

    // 附加共享内存
    SharedTopKData* data = (SharedTopKData*)shmat(shmid, nullptr, 0);
    if (data == (void*)-1) {
        return nullptr;
    }

    return std::shared_ptr<SharedTopK>(new SharedTopK(key, shmid, data, false));
}

SharedTopK::SharedTopK(key_t key, int shmid, SharedTopKData* data, bool created)
    : shm_key_(key), shmid_(shmid), data_(data), created_(created), valid_(true) {
}

SharedTopK::~SharedTopK() {
    if (data_ && data_ != (void*)-1) {
        shmdt(data_);
    }
    
    if (created_ && shmid_ != -1) {
        shmctl(shmid_, IPC_RMID, nullptr);
    }
}

bool SharedTopK::init_semaphores() {
    if (!data_) return false;

    // 初始化信号量
    if (sem_init(&data_->sem_mutex, 1, 1) != 0) {
        return false;
    }
    
    if (sem_init(&data_->sem_empty, 1, data_->k) != 0) {
        sem_destroy(&data_->sem_mutex);
        return false;
    }
    
    if (sem_init(&data_->sem_full, 1, 0) != 0) {
        sem_destroy(&data_->sem_mutex);
        sem_destroy(&data_->sem_empty);
        return false;
    }

    return true;
}

void SharedTopK::wait_sem(sem_t* sem) {
    while (sem_wait(sem) == -1 && errno == EINTR) {
        // 被信号中断，继续等待
    }
}

void SharedTopK::post_sem(sem_t* sem) {
    sem_post(sem);
}

void SharedTopK::lock() const {
    const_cast<SharedTopK*>(this)->wait_sem(&data_->sem_mutex);
}

void SharedTopK::unlock() const {
    const_cast<SharedTopK*>(this)->post_sem(&data_->sem_mutex);
}

bool SharedTopK::add(const std::string& host_id, double value) {
    if (!data_ || !valid_) return false;

    lock();
    
    // 清理过期数据
    internal_cleanup();

    int64_t now = std::chrono::duration_cast<std::chrono::seconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();

    // 查找是否已存在
    int existing_index = -1;
    for (int i = 0; i < data_->count; i++) {
        if (data_->items[i].valid && strcmp(data_->items[i].host_id, host_id.c_str()) == 0) {
            existing_index = i;
            break;
        }
    }

    if (existing_index >= 0) {
        // 更新现有项
        data_->items[existing_index].value = value;
        data_->items[existing_index].timestamp = now;
    } else {
        // 添加新项
        if (data_->count < data_->k) {
            // 还有空间，直接添加
            strncpy(data_->items[data_->count].host_id, host_id.c_str(), sizeof(data_->items[data_->count].host_id) - 1);
            data_->items[data_->count].host_id[sizeof(data_->items[data_->count].host_id) - 1] = '\0';
            data_->items[data_->count].value = value;
            data_->items[data_->count].timestamp = now;
            data_->items[data_->count].valid = 1;
            data_->count++;
        } else {
            // 找到最小值替换
            int min_index = 0;
            double min_value = data_->items[0].value;
            for (int i = 1; i < data_->count; i++) {
                if (data_->items[i].valid && data_->items[i].value < min_value) {
                    min_value = data_->items[i].value;
                    min_index = i;
                }
            }
            
            if (value > min_value) {
                strncpy(data_->items[min_index].host_id, host_id.c_str(), sizeof(data_->items[min_index].host_id) - 1);
                data_->items[min_index].host_id[sizeof(data_->items[min_index].host_id) - 1] = '\0';
                data_->items[min_index].value = value;
                data_->items[min_index].timestamp = now;
                data_->items[min_index].valid = 1;
            }
        }
    }

    unlock();
    return true;
}

std::vector<TopKItem> SharedTopK::get_top_k() {
    std::vector<TopKItem> result;
    
    if (!data_ || !valid_) return result;

    lock();
    
    // 清理过期数据
    internal_cleanup();

    // 复制有效数据
    for (int i = 0; i < data_->count; i++) {
        if (data_->items[i].valid) {
            result.emplace_back(
                data_->items[i].host_id,
                data_->items[i].value,
                data_->items[i].timestamp
            );
        }
    }

    unlock();

    // 按值降序排序
    std::sort(result.begin(), result.end(), 
        [](const TopKItem& a, const TopKItem& b) { return a.value > b.value; });

    return result;
}

int SharedTopK::get_count() const {
    if (!data_ || !valid_) return 0;

    lock();
    int count = data_->count;
    unlock();
    return count;
}

void SharedTopK::clear() {
    if (!data_ || !valid_) return;

    lock();
    data_->count = 0;
    for (int i = 0; i < data_->k; i++) {
        data_->items[i].valid = 0;
    }
    unlock();
}

void SharedTopK::cleanup_expired() const {
    if (!data_ || !valid_) return;

    lock();
    internal_cleanup();
    unlock();
}

void SharedTopK::internal_cleanup() const {
    if (!data_) return;

    int64_t now = std::chrono::duration_cast<std::chrono::seconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();
    int64_t expire_time = now - data_->ttl_seconds;

    // 移除过期项
    int write_index = 0;
    for (int i = 0; i < data_->count; i++) {
        if (data_->items[i].valid && data_->items[i].timestamp >= expire_time) {
            if (write_index != i) {
                data_->items[write_index] = data_->items[i];
            }
            write_index++;
        }
    }
    
    // 清理剩余项
    for (int i = write_index; i < data_->count; i++) {
        data_->items[i].valid = 0;
    }
    
    data_->count = write_index;
}

void SharedTopK::set_ttl(int seconds) {
    if (!data_ || !valid_) return;

    lock();
    data_->ttl_seconds = seconds;
    unlock();
}

bool SharedTopK::is_valid() const {
    return valid_ && data_ && data_->initialized;
}

// 批量添加元素
bool SharedTopK::add_batch(const std::vector<std::pair<std::string, double>>& items) {
    if (!data_ || !valid_) return false;

    lock();
    
    // 清理过期数据
    internal_cleanup();

    int64_t now = std::chrono::duration_cast<std::chrono::seconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();

    for (const auto& item : items) {
        // 查找是否已存在
        int existing_index = -1;
        for (int i = 0; i < data_->count; i++) {
            if (data_->items[i].valid && strcmp(data_->items[i].host_id, item.first.c_str()) == 0) {
                existing_index = i;
                break;
            }
        }

        if (existing_index >= 0) {
            // 更新现有项
            data_->items[existing_index].value = item.second;
            data_->items[existing_index].timestamp = now;
            data_->items[existing_index].update_count++;
        } else {
            // 添加新项
            if (data_->count < data_->k) {
                // 还有空间，直接添加
                strncpy(data_->items[data_->count].host_id, item.first.c_str(), sizeof(data_->items[data_->count].host_id) - 1);
                data_->items[data_->count].host_id[sizeof(data_->items[data_->count].host_id) - 1] = '\0';
                data_->items[data_->count].value = item.second;
                data_->items[data_->count].timestamp = now;
                data_->items[data_->count].valid = 1;
                data_->items[data_->count].update_count = 1;
                data_->count++;
            } else {
                // 找到最小值替换
                int min_index = 0;
                double min_value = data_->items[0].value;
                for (int i = 1; i < data_->count; i++) {
                    if (data_->items[i].valid && data_->items[i].value < min_value) {
                        min_value = data_->items[i].value;
                        min_index = i;
                    }
                }
                
                if (item.second > min_value) {
                    strncpy(data_->items[min_index].host_id, item.first.c_str(), sizeof(data_->items[min_index].host_id) - 1);
                    data_->items[min_index].host_id[sizeof(data_->items[min_index].host_id) - 1] = '\0';
                    data_->items[min_index].value = item.second;
                    data_->items[min_index].timestamp = now;
                    data_->items[min_index].valid = 1;
                    data_->items[min_index].update_count = 1;
                }
            }
        }
    }

    // 更新统计信息
    data_->total_operations += items.size();
    data_->total_updates += items.size();

    unlock();
    return true;
}

// 获取统计信息
void SharedTopK::get_stats(int& total_updates, int& total_operations) const {
    if (!data_ || !valid_) {
        total_updates = 0;
        total_operations = 0;
        return;
    }

    lock();
    total_updates = data_->total_updates;
    total_operations = data_->total_operations;
    unlock();
}

// 获取特定主机的值
bool SharedTopK::get_value(const std::string& host_id, double& value) const {
    if (!data_ || !valid_) return false;

    lock();
    
    for (int i = 0; i < data_->count; i++) {
        if (data_->items[i].valid && strcmp(data_->items[i].host_id, host_id.c_str()) == 0) {
            value = data_->items[i].value;
            unlock();
            return true;
        }
    }

    unlock();
    return false;
}

// 移除特定主机
bool SharedTopK::remove(const std::string& host_id) {
    if (!data_ || !valid_) return false;

    lock();
    
    bool found = false;
    for (int i = 0; i < data_->count; i++) {
        if (data_->items[i].valid && strcmp(data_->items[i].host_id, host_id.c_str()) == 0) {
            data_->items[i].valid = 0;
            found = true;
            break;
        }
    }

    if (found) {
        // 重新整理数组
        int write_index = 0;
        for (int i = 0; i < data_->count; i++) {
            if (data_->items[i].valid) {
                if (write_index != i) {
                    data_->items[write_index] = data_->items[i];
                }
                write_index++;
            }
        }
        
        for (int i = write_index; i < data_->count; i++) {
            data_->items[i].valid = 0;
        }
        
        data_->count = write_index;
        data_->total_operations++;
    }

    unlock();
    return found;
}

// FastTopK 实现
FastTopK::FastTopK(int k, int ttl_seconds) : k_(k), ttl_seconds_(ttl_seconds) {
    min_heap_.reserve(k);
}

FastTopK::~FastTopK() {
}

void FastTopK::add(const std::string& host_id, double value) {
    int64_t now = std::chrono::duration_cast<std::chrono::seconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();

    auto it = item_map_.find(host_id);
    if (it != item_map_.end()) {
        // 更新现有项
        int index = it->second;
        min_heap_[index].value = value;
        min_heap_[index].timestamp = now;
        heapify_down(index);
        heapify_up(index);
    } else {
        // 添加新项
        if (min_heap_.size() < static_cast<size_t>(k_)) {
            min_heap_.emplace_back(host_id, value, now);
            item_map_[host_id] = min_heap_.size() - 1;
            heapify_up(min_heap_.size() - 1);
        } else {
            // 替换最小值
            if (value > min_heap_[0].value) {
                item_map_.erase(min_heap_[0].host_id);
                min_heap_[0] = Item(host_id, value, now);
                item_map_[host_id] = 0;
                heapify_down(0);
            }
        }
    }

    // 清理过期数据
    cleanup_expired();
}

std::vector<TopKItem> FastTopK::get_top_k() const {
    cleanup_expired();

    std::vector<TopKItem> result;
    for (const auto& item : min_heap_) {
        result.emplace_back(item.host_id, item.value, item.timestamp);
    }

    // 按值降序排序
    std::sort(result.begin(), result.end(), 
        [](const TopKItem& a, const TopKItem& b) { return a.value > b.value; });

    return result;
}

int FastTopK::get_count() const {
    cleanup_expired();
    return min_heap_.size();
}

void FastTopK::clear() {
    min_heap_.clear();
    item_map_.clear();
}

void FastTopK::set_ttl(int seconds) {
    ttl_seconds_ = seconds;
}

int FastTopK::get_ttl() const {
    return ttl_seconds_;
}

int FastTopK::get_k() const {
    return k_;
}

int FastTopK::set_k(int k) {
    if (k <= 0) return -1;
    k_ = k;
    return 0;
}

bool FastTopK::contains(const std::string& host_id) const {
    return item_map_.find(host_id) != item_map_.end();
}

int FastTopK::get_expired_count() const {
    auto now = std::chrono::duration_cast<std::chrono::seconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();
    
    int expired = 0;
    for (const auto& item : min_heap_) {
        if (now - item.timestamp > ttl_seconds_) {
            expired++;
        }
    }
    return expired;
}

void FastTopK::heapify_up(int index) {
    while (index > 0) {
        int parent = (index - 1) / 2;
        if (min_heap_[index] < min_heap_[parent]) {
            std::swap(min_heap_[index], min_heap_[parent]);
            item_map_[min_heap_[index].host_id] = index;
            item_map_[min_heap_[parent].host_id] = parent;
            index = parent;
        } else {
            break;
        }
    }
}

void FastTopK::heapify_down(int index) {
    int size = min_heap_.size();
    while (true) {
        int left = 2 * index + 1;
        int right = 2 * index + 2;
        int smallest = index;

        if (left < size && min_heap_[left] < min_heap_[smallest]) {
            smallest = left;
        }
        if (right < size && min_heap_[right] < min_heap_[smallest]) {
            smallest = right;
        }

        if (smallest != index) {
            std::swap(min_heap_[index], min_heap_[smallest]);
            item_map_[min_heap_[index].host_id] = index;
            item_map_[min_heap_[smallest].host_id] = smallest;
            index = smallest;
        } else {
            break;
        }
    }
}

void FastTopK::cleanup_expired() const {
    if (ttl_seconds_ <= 0) return;

    int64_t now = std::chrono::duration_cast<std::chrono::seconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();
    int64_t expire_time = now - ttl_seconds_;

    std::vector<int> to_remove;
    for (size_t i = 0; i < min_heap_.size(); i++) {
        if (min_heap_[i].timestamp < expire_time) {
            to_remove.push_back(i);
        }
    }

    // 从后往前移除，避免索引问题
    std::sort(to_remove.rbegin(), to_remove.rend());
    for (int index : to_remove) {
        const_cast<std::unordered_map<std::string, int>*>(&item_map_)->erase(min_heap_[index].host_id);
        if (index != static_cast<int>(min_heap_.size()) - 1) {
            const_cast<std::vector<Item>*>(&min_heap_)->operator[](index) = min_heap_.back();
            const_cast<std::unordered_map<std::string, int>*>(&item_map_)->operator[](min_heap_[index].host_id) = index;
            const_cast<FastTopK*>(this)->heapify_down(index);
        }
        const_cast<std::vector<Item>*>(&min_heap_)->pop_back();
    }
}

bool FastTopK::sync_to_shared(const std::shared_ptr<SharedTopK>& shared) {
    if (!shared) return false;

    cleanup_expired();
    
    for (const auto& item : min_heap_) {
        shared->add(item.host_id, item.value);
    }

    return true;
}

bool FastTopK::sync_from_shared(const std::shared_ptr<SharedTopK>& shared) {
    if (!shared) return false;

    auto items = shared->get_top_k();
    clear();
    
    for (const auto& item : items) {
        add(item.host_id, item.value);
    }

    return true;
}

// 批量添加元素
void FastTopK::add_batch(const std::vector<std::pair<std::string, double>>& items) {
    for (const auto& item : items) {
        add(item.first, item.second);
    }
}

// 获取Top-K元素（带过滤）
std::vector<TopKItem> FastTopK::get_top_k_filtered(const std::function<bool(const TopKItem&)>& filter) const {
    cleanup_expired();

    std::vector<TopKItem> result;
    for (const auto& item : min_heap_) {
        TopKItem topk_item(item.host_id, item.value, item.timestamp, 0);
        if (filter(topk_item)) {
            result.emplace_back(item.host_id, item.value, item.timestamp, 0);
        }
    }

    // 按值降序排序
    std::sort(result.begin(), result.end(), 
        [](const TopKItem& a, const TopKItem& b) { return a.value > b.value; });

    return result;
}

// 获取特定主机的值
bool FastTopK::get_value(const std::string& host_id, double& value) const {
    auto it = item_map_.find(host_id);
    if (it != item_map_.end()) {
        value = min_heap_[it->second].value;
        return true;
    }
    return false;
}

// 移除特定主机
bool FastTopK::remove(const std::string& host_id) {
    auto it = item_map_.find(host_id);
    if (it == item_map_.end()) {
        return false;
    }

    int index = it->second;
    
    // 如果不是最后一个元素，交换到最后
    if (index != static_cast<int>(min_heap_.size()) - 1) {
        min_heap_[index] = min_heap_.back();
        item_map_[min_heap_[index].host_id] = index;
    }
    
    // 移除元素
    min_heap_.pop_back();
    item_map_.erase(it);
    
    // 重新调整堆
    if (!min_heap_.empty()) {
        heapify_down(index);
    }
    
    return true;
}

// 获取内存使用情况
size_t FastTopK::get_memory_usage() const {
    size_t usage = sizeof(FastTopK);
    usage += min_heap_.capacity() * sizeof(Item);
    usage += item_map_.bucket_count() * (sizeof(std::string) + sizeof(int));
    return usage;
}

// 导出为JSON字符串
std::string FastTopK::to_json() const {
    std::vector<TopKItem> items = get_top_k();
    
    std::string json = "[";
    for (size_t i = 0; i < items.size(); ++i) {
        if (i > 0) json += ",";
        json += "{\"host_id\":\"" + items[i].host_id + "\",";
        json += "\"value\":" + std::to_string(items[i].value) + ",";
        json += "\"timestamp\":" + std::to_string(items[i].timestamp) + ",";
        json += "\"update_count\":" + std::to_string(items[i].update_count) + "}";
    }
    json += "]";
    
    return json;
}

// 获取统计信息
void FastTopK::get_stats(int64_t& total_items, int& active_items, int& expired_items, 
                        int64_t& total_operations, int64_t& uptime, int64_t& memory_usage) const {
    total_items = min_heap_.size();
    active_items = min_heap_.size() - get_expired_count();
    expired_items = get_expired_count();
    total_operations = 0; // 简化实现
    uptime = 0; // 简化实现
    memory_usage = get_memory_usage();
}

// 启用调试模式
void FastTopK::enable_debug(bool enable) {
    // 简化实现 - 可以添加日志输出
    (void)enable;
}

// TopKManager 实现
TopKManager& TopKManager::get_instance() {
    static TopKManager instance;
    return instance;
}

TopKManager::TopKManager() : initialized_(false), running_(false) {
}

TopKManager::~TopKManager() {
    shutdown();
}

bool TopKManager::initialize(const std::string& shm_key, int k, int ttl_seconds) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (initialized_) {
        return true;
    }

    // 创建共享内存Top-K
    shared_topk_ = SharedTopK::create(shm_key, k, ttl_seconds);
    if (!shared_topk_) {
        // 尝试附加到现有的
        shared_topk_ = SharedTopK::attach(shm_key);
        if (!shared_topk_) {
            return false;
        }
    }

    // 创建默认的FastTopK实例
    topk_instances_["default"] = std::make_shared<FastTopK>(k, ttl_seconds);

    // 启动清理线程
    running_ = true;
    cleanup_thread_ = std::thread(&TopKManager::cleanup_thread, this);

    initialized_ = true;
    return true;
}

void TopKManager::add_metric(const std::string& host_id, const std::string& metric_name, double value) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (!initialized_) return;

    // 查找或创建Top-K实例
    auto it = topk_instances_.find(metric_name);
    if (it == topk_instances_.end()) {
        int k = 10;
        int ttl = 3600;
        if (shared_topk_) {
            k = shared_topk_->get_count();
            if (k <= 0) k = 10;
        }
        topk_instances_[metric_name] = std::make_shared<FastTopK>(k, ttl);
        it = topk_instances_.find(metric_name);
    }

    // 添加数据
    it->second->add(host_id, value);

    // 同步到共享内存
    if (shared_topk_ && metric_name == "default") {
        it->second->sync_to_shared(shared_topk_);
    }
}

// 批量添加数据
void TopKManager::add_metric_batch(const std::string& metric_name, const std::vector<std::pair<std::string, double>>& items) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (!initialized_) return;

    // 查找或创建Top-K实例
    auto it = topk_instances_.find(metric_name);
    if (it == topk_instances_.end()) {
        int k = 10;
        int ttl = 3600;
        if (shared_topk_) {
            k = shared_topk_->get_count();
            if (k <= 0) k = 10;
        }
        topk_instances_[metric_name] = std::make_shared<FastTopK>(k, ttl);
        it = topk_instances_.find(metric_name);
    }

    // 批量添加数据
    it->second->add_batch(items);

    // 同步到共享内存
    if (shared_topk_ && metric_name == "default") {
        it->second->sync_to_shared(shared_topk_);
    }
}

// 获取系统统计信息
void TopKManager::get_system_stats(int& total_metrics, int& total_items, int& total_updates) const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    total_metrics = topk_instances_.size();
    total_items = 0;
    total_updates = 0;
    
    for (const auto& pair : topk_instances_) {
        total_items += pair.second->get_count();
    }
    
    if (shared_topk_) {
        int updates, operations;
        shared_topk_->get_stats(updates, operations);
        total_updates = updates;
    }
}

// 创建新的指标实例
bool TopKManager::create_metric(const std::string& metric_name, int k, int ttl_seconds) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (!initialized_) return false;
    
    if (topk_instances_.find(metric_name) != topk_instances_.end()) {
        return false; // 已存在
    }
    
    topk_instances_[metric_name] = std::make_shared<FastTopK>(k, ttl_seconds);
    return true;
}

// 移除指标
bool TopKManager::remove_metric(const std::string& metric_name) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (!initialized_) return false;
    
    auto it = topk_instances_.find(metric_name);
    if (it == topk_instances_.end()) {
        return false;
    }
    
    topk_instances_.erase(it);
    return true;
}

std::vector<TopKItem> TopKManager::get_top_k(const std::string& metric_name) const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (!initialized_) return {};

    auto it = topk_instances_.find(metric_name);
    if (it != topk_instances_.end()) {
        return it->second->get_top_k();
    }

    return {};
}

std::vector<std::string> TopKManager::get_metric_names() const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    std::vector<std::string> names;
    for (const auto& pair : topk_instances_) {
        names.push_back(pair.first);
    }
    return names;
}

void TopKManager::shutdown() {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (!initialized_) return;

    running_ = false;
    cv_.notify_all();
    
    if (cleanup_thread_.joinable()) {
        cleanup_thread_.join();
    }

    topk_instances_.clear();
    shared_topk_.reset();
    initialized_ = false;
}

void TopKManager::cleanup_thread() {
    while (running_) {
        std::unique_lock<std::mutex> lock(mutex_);
        
        // 每5分钟清理一次
        if (cv_.wait_for(lock, std::chrono::minutes(5), [this] { return !running_; })) {
            break;
        }

        if (!running_) break;

        // 清理所有实例的过期数据
        for (auto& pair : topk_instances_) {
            pair.second->cleanup_expired();
        }

        if (shared_topk_) {
            shared_topk_->cleanup_expired();
        }
    }
}