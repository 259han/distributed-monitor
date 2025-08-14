#include "streaming_topk.h"
#include <any>
#include <cassert>
#include <sstream>

// StreamingTopKProcessor 实现

StreamingTopKProcessor::StreamingTopKProcessor(int k, int ttl_seconds)
    : k_(k), ttl_seconds_(ttl_seconds), closed_(false), update_count_(0), last_update_(std::chrono::system_clock::now()) {
    assert(k > 0 && ttl_seconds > 0);
    min_heap_.reserve(k);
}

StreamingTopKProcessor::~StreamingTopKProcessor() {
    close();
}

void StreamingTopKProcessor::set_update_callback(UpdateCallback callback) {
    std::lock_guard<std::mutex> lock(mutex_);
    update_callback_ = callback;
}

void StreamingTopKProcessor::set_expire_callback(ExpireCallback callback) {
    std::lock_guard<std::mutex> lock(mutex_);
    expire_callback_ = callback;
}

void StreamingTopKProcessor::process_stream(const std::string& host_id, double value) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return;
    }

    auto now = std::chrono::system_clock::now();
    update_count_++;
    last_update_ = now;

    // 清理过期数据
    cleanup_expired(now);

    // 检查是否已存在
    auto it = item_map_.find(host_id);
    if (it != item_map_.end()) {
        // 更新现有项
        int index = it->second;
        min_heap_[index].value = value;
        min_heap_[index].timestamp = std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count();
        min_heap_[index].update_count++;
        
        // 调整堆
        heapify_down(index);
        heapify_up(index);
    } else {
        // 创建新项
        Item new_item(host_id, value, 
                     std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count(), 
                     -1, 1);

        // 流式处理：立即决定是否加入Top-K
        if (static_cast<int>(min_heap_.size()) < k_) {
            // 堆未满，直接添加
            new_item.index = min_heap_.size();
            min_heap_.push_back(new_item);
            item_map_[host_id] = new_item.index;
            heapify_up(new_item.index);
        } else if (value > min_heap_[0].value) {
            // 新值大于最小值，替换
            std::string old_host_id = min_heap_[0].host_id;
            
            // 移除最小项
            item_map_.erase(old_host_id);
            min_heap_[0] = new_item;
            min_heap_[0].index = 0;
            item_map_[host_id] = 0;
            
            // 触发过期回调
            trigger_expire_callback(old_host_id);
            
            // 调整堆
            heapify_down(0);
        }
        // 如果新值小于等于最小值，直接丢弃（流式处理的核心）
    }

    // 触发更新回调
    trigger_update_callback();
}

void StreamingTopKProcessor::process_stream_batch(const std::vector<std::pair<std::string, double>>& items) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return;
    }

    auto now = std::chrono::system_clock::now();
    update_count_ += items.size();
    last_update_ = now;

    // 清理过期数据
    cleanup_expired(now);

    // 批量处理
    for (const auto& item : items) {
        const std::string& host_id = item.first;
        double value = item.second;

        auto it = item_map_.find(host_id);
        if (it != item_map_.end()) {
            // 更新现有项
            int index = it->second;
            min_heap_[index].value = value;
            min_heap_[index].timestamp = std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count();
            min_heap_[index].update_count++;
            
            heapify_down(index);
            heapify_up(index);
        } else {
            // 创建新项
            Item new_item(host_id, value, 
                         std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count(), 
                         -1, 1);

            // 流式决策
            if (static_cast<int>(min_heap_.size()) < k_) {
                new_item.index = min_heap_.size();
                min_heap_.push_back(new_item);
                item_map_[host_id] = new_item.index;
                heapify_up(new_item.index);
            } else if (value > min_heap_[0].value) {
                std::string old_host_id = min_heap_[0].host_id;
                
                item_map_.erase(old_host_id);
                min_heap_[0] = new_item;
                min_heap_[0].index = 0;
                item_map_[host_id] = 0;
                
                trigger_expire_callback(old_host_id);
                heapify_down(0);
            }
        }
    }

    // 触发更新回调
    trigger_update_callback();
}

std::vector<StreamingTopKItem> StreamingTopKProcessor::get_top_k() const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return {};
    }

    return get_top_k_internal();
}

std::vector<StreamingTopKItem> StreamingTopKProcessor::get_top_k_filtered(
    const std::function<bool(const StreamingTopKItem&)>& filter) const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return {};
    }

    auto results = get_top_k_internal();
    if (!filter) {
        return results;
    }

    std::vector<StreamingTopKItem> filtered;
    for (const auto& item : results) {
        if (filter(item)) {
            filtered.push_back(item);
        }
    }

    return filtered;
}

std::unordered_map<std::string, std::any> StreamingTopKProcessor::get_stats() const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    std::unordered_map<std::string, std::any> stats;
    stats["total_updates"] = update_count_.load();
    stats["current_items"] = static_cast<int>(min_heap_.size());
    stats["k_value"] = k_;
    stats["ttl_seconds"] = ttl_seconds_;
    stats["last_update"] = last_update_;
    stats["is_closed"] = closed_.load();
    
    return stats;
}

int StreamingTopKProcessor::get_count() const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return 0;
    }

    return static_cast<int>(min_heap_.size());
}

bool StreamingTopKProcessor::contains(const std::string& host_id) const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return false;
    }

    return item_map_.find(host_id) != item_map_.end();
}

bool StreamingTopKProcessor::get_value(const std::string& host_id, double& value) const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return false;
    }

    auto it = item_map_.find(host_id);
    if (it != item_map_.end()) {
        value = min_heap_[it->second].value;
        return true;
    }
    return false;
}

void StreamingTopKProcessor::clear() {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return;
    }

    // 触发过期回调
    if (expire_callback_) {
        for (const auto& item : min_heap_) {
            expire_callback_(item.host_id);
        }
    }

    item_map_.clear();
    min_heap_.clear();
    update_count_ = 0;
}

void StreamingTopKProcessor::set_ttl(int seconds) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return;
    }

    ttl_seconds_ = seconds;
}

int StreamingTopKProcessor::get_ttl() const {
    return ttl_seconds_;
}

int StreamingTopKProcessor::get_k() const {
    return k_;
}

void StreamingTopKProcessor::set_k(int k) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load() || k <= 0) {
        return;
    }

    k_ = k;
    
    // 如果新的k值小于当前堆大小，需要移除多余的元素
    while (static_cast<int>(min_heap_.size()) > k_) {
        std::string old_host_id = min_heap_[0].host_id;
        item_map_.erase(old_host_id);
        
        // 将最后一个元素移到顶部
        min_heap_[0] = min_heap_.back();
        min_heap_[0].index = 0;
        item_map_[min_heap_[0].host_id] = 0;
        min_heap_.pop_back();
        
        trigger_expire_callback(old_host_id);
        heapify_down(0);
    }
}

void StreamingTopKProcessor::close() {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (closed_.load()) {
        return;
    }

    // 触发过期回调
    if (expire_callback_) {
        for (const auto& item : min_heap_) {
            expire_callback_(item.host_id);
        }
    }

    item_map_.clear();
    min_heap_.clear();
    closed_ = true;
}

bool StreamingTopKProcessor::is_closed() const {
    return closed_.load();
}

// 私有方法实现

void StreamingTopKProcessor::heapify_up(int index) {
    while (index > 0) {
        int parent = (index - 1) / 2;
        if (min_heap_[index] < min_heap_[parent]) {
            std::swap(min_heap_[index], min_heap_[parent]);
            min_heap_[index].index = index;
            min_heap_[parent].index = parent;
            item_map_[min_heap_[index].host_id] = index;
            item_map_[min_heap_[parent].host_id] = parent;
            index = parent;
        } else {
            break;
        }
    }
}

void StreamingTopKProcessor::heapify_down(int index) {
    int size = static_cast<int>(min_heap_.size());
    while (true) {
        int smallest = index;
        int left = 2 * index + 1;
        int right = 2 * index + 2;

        if (left < size && min_heap_[left] < min_heap_[smallest]) {
            smallest = left;
        }
        if (right < size && min_heap_[right] < min_heap_[smallest]) {
            smallest = right;
        }

        if (smallest != index) {
            std::swap(min_heap_[index], min_heap_[smallest]);
            min_heap_[index].index = index;
            min_heap_[smallest].index = smallest;
            item_map_[min_heap_[index].host_id] = index;
            item_map_[min_heap_[smallest].host_id] = smallest;
            index = smallest;
        } else {
            break;
        }
    }
}

void StreamingTopKProcessor::cleanup_expired(const std::chrono::system_clock::time_point& now) {
    int64_t expire_time = std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count() - ttl_seconds_;
    
    // 从堆顶开始检查过期项
    while (!min_heap_.empty()) {
        if (min_heap_[0].timestamp < expire_time) {
            // 移除过期项
            std::string old_host_id = min_heap_[0].host_id;
            item_map_.erase(old_host_id);
            
            if (min_heap_.size() > 1) {
                min_heap_[0] = min_heap_.back();
                min_heap_[0].index = 0;
                item_map_[min_heap_[0].host_id] = 0;
                min_heap_.pop_back();
                heapify_down(0);
            } else {
                min_heap_.pop_back();
            }
            
            trigger_expire_callback(old_host_id);
        } else {
            break; // 堆顶未过期，其他项也不会过期
        }
    }
}

std::vector<StreamingTopKItem> StreamingTopKProcessor::get_top_k_internal() const {
    std::vector<StreamingTopKItem> results;
    results.reserve(min_heap_.size());
    
    for (const auto& item : min_heap_) {
        results.emplace_back(item.host_id, item.value, item.timestamp, item.index, item.update_count);
    }

    // 按值从大到小排序
    std::sort(results.begin(), results.end(), 
              [](const StreamingTopKItem& a, const StreamingTopKItem& b) {
                  return a.value > b.value;
              });

    return results;
}

void StreamingTopKProcessor::trigger_update_callback() {
    if (update_callback_) {
        auto results = get_top_k_internal();
        update_callback_(results);
    }
}

void StreamingTopKProcessor::trigger_expire_callback(const std::string& host_id) {
    if (expire_callback_) {
        expire_callback_(host_id);
    }
}

// StreamingTopKManager 实现

StreamingTopKManager& StreamingTopKManager::get_instance() {
    static StreamingTopKManager instance;
    return instance;
}

StreamingTopKManager::StreamingTopKManager()
    : initialized_(false), default_k_(10), default_ttl_(3600), running_(false) {
}

StreamingTopKManager::~StreamingTopKManager() {
    shutdown();
}

bool StreamingTopKManager::initialize(int default_k, int default_ttl) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (initialized_) {
        return true;
    }

    default_k_ = default_k;
    default_ttl_ = default_ttl;
    
    // 启动清理线程
    running_ = true;
    cleanup_thread_ = std::thread(&StreamingTopKManager::cleanup_thread, this);
    
    initialized_ = true;
    return true;
}

std::shared_ptr<StreamingTopKProcessor> StreamingTopKManager::create_processor(
    const std::string& name, int k, int ttl_seconds) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    if (!initialized_) {
        return nullptr;
    }

    auto processor = std::make_shared<StreamingTopKProcessor>(k, ttl_seconds);
    
    // 设置全局回调
    if (global_update_callback_) {
        processor->set_update_callback(global_update_callback_);
    }
    if (global_expire_callback_) {
        processor->set_expire_callback(global_expire_callback_);
    }
    
    processors_[name] = processor;
    return processor;
}

std::shared_ptr<StreamingTopKProcessor> StreamingTopKManager::get_processor(const std::string& name) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    auto it = processors_.find(name);
    if (it != processors_.end()) {
        return it->second;
    }
    return nullptr;
}

bool StreamingTopKManager::remove_processor(const std::string& name) {
    std::lock_guard<std::mutex> lock(mutex_);
    
    auto it = processors_.find(name);
    if (it != processors_.end()) {
        it->second->close();
        processors_.erase(it);
        return true;
    }
    return false;
}

void StreamingTopKManager::add_metric(const std::string& processor_name, const std::string& host_id, double value) {
    auto processor = get_processor(processor_name);
    if (processor) {
        processor->process_stream(host_id, value);
    }
}

void StreamingTopKManager::add_metric_batch(const std::string& processor_name, 
                                          const std::vector<std::pair<std::string, double>>& items) {
    auto processor = get_processor(processor_name);
    if (processor) {
        processor->process_stream_batch(items);
    }
}

std::vector<StreamingTopKItem> StreamingTopKManager::get_top_k(const std::string& processor_name) const {
    std::lock_guard<std::mutex> lock(mutex_);
    auto it = processors_.find(processor_name);
    if (it != processors_.end()) {
        return it->second->get_top_k();
    }
    return {};
}

std::vector<std::string> StreamingTopKManager::get_processor_names() const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    std::vector<std::string> names;
    names.reserve(processors_.size());
    
    for (const auto& pair : processors_) {
        names.push_back(pair.first);
    }
    
    return names;
}

std::unordered_map<std::string, std::any> StreamingTopKManager::get_system_stats() const {
    std::lock_guard<std::mutex> lock(mutex_);
    
    std::unordered_map<std::string, std::any> stats;
    stats["total_processors"] = static_cast<int>(processors_.size());
    stats["initialized"] = initialized_;
    stats["default_k"] = default_k_;
    stats["default_ttl"] = default_ttl_;
    stats["running"] = running_.load();
    
    int total_updates = 0;
    int total_items = 0;
    
    for (const auto& pair : processors_) {
        auto processor_stats = pair.second->get_stats();
        total_updates += std::any_cast<int64_t>(processor_stats["total_updates"]);
        total_items += std::any_cast<int>(processor_stats["current_items"]);
    }
    
    stats["total_updates"] = total_updates;
    stats["total_items"] = total_items;
    
    return stats;
}

void StreamingTopKManager::shutdown() {
    {
        std::lock_guard<std::mutex> lock(mutex_);
        
        if (!initialized_) {
            return;
        }

        // 停止清理线程
        running_ = false;
        cv_.notify_all();
    }
    
    if (cleanup_thread_.joinable()) {
        cleanup_thread_.join();
    }
    
    // 关闭所有处理器
    std::lock_guard<std::mutex> lock(mutex_);
    for (auto& pair : processors_) {
        pair.second->close();
    }
    processors_.clear();
    initialized_ = false;
}

void StreamingTopKManager::set_global_update_callback(StreamingTopKProcessor::UpdateCallback callback) {
    std::lock_guard<std::mutex> lock(mutex_);
    global_update_callback_ = callback;
    
    // 为现有处理器设置回调
    for (auto& pair : processors_) {
        pair.second->set_update_callback(callback);
    }
}

void StreamingTopKManager::set_global_expire_callback(StreamingTopKProcessor::ExpireCallback callback) {
    std::lock_guard<std::mutex> lock(mutex_);
    global_expire_callback_ = callback;
    
    // 为现有处理器设置回调
    for (auto& pair : processors_) {
        pair.second->set_expire_callback(callback);
    }
}

void StreamingTopKManager::cleanup_thread() {
    while (running_.load()) {
        {
            std::unique_lock<std::mutex> lock(mutex_);
            cv_.wait_for(lock, std::chrono::seconds(60), [this] { return !running_.load(); });
        }
        
        if (!running_.load()) {
            break;
        }
        
        // 执行清理任务
        std::lock_guard<std::mutex> lock(mutex_);
        for (auto& processor_pair : processors_) {
            // 这里可以添加额外的清理逻辑
            // 比如清理长时间未更新的处理器等
            (void)processor_pair; // 避免未使用变量警告
        }
    }
}
