#ifdef COMPILE_TEST
#include "topk_shared.h"
#include <iostream>
#include <vector>
#include <string>
#include <chrono>
#include <thread>

int main() {
    std::cout << "开始Top-K模块测试..." << std::endl;
    
    // 测试FastTopK
    FastTopK topk(5, 60); // K=5, TTL=60秒
    
    // 添加测试数据
    std::vector<std::pair<std::string, double>> test_data = {
        {"host1", 85.5},
        {"host2", 92.3},
        {"host3", 78.9},
        {"host4", 96.7},
        {"host5", 81.2},
        {"host6", 88.8},
        {"host7", 75.4},
        {"host8", 94.1}
    };
    
    for (const auto& item : test_data) {
        topk.add(item.first, item.second);
        std::cout << "✓ 添加数据: " << item.first << " = " << item.second << std::endl;
    }
    
    // 获取Top-K结果
    auto results = topk.get_top_k();
    std::cout << "Top-K结果:" << std::endl;
    for (size_t i = 0; i < results.size(); i++) {
        std::cout << i+1 << ". " << results[i].host_id << ": " << results[i].value << std::endl;
    }
    
    // 测试共享内存
    auto shared_topk = SharedTopK::create("/tmp/topk_test", 5, 60);
    if (shared_topk) {
        std::cout << "✓ 共享内存Top-K创建成功" << std::endl;
        
        // 同步数据到共享内存
        if (topk.sync_to_shared(shared_topk)) {
            std::cout << "✓ 数据同步到共享内存成功" << std::endl;
        }
        
        // 从共享内存读取
        auto shared_results = shared_topk->get_top_k();
        std::cout << "共享内存Top-K结果:" << std::endl;
        for (size_t i = 0; i < shared_results.size(); i++) {
            std::cout << i+1 << ". " << shared_results[i].host_id << ": " << shared_results[i].value << std::endl;
        }
    } else {
        std::cout << "✗ 共享内存Top-K创建失败" << std::endl;
    }
    
    std::cout << "✓ Top-K模块测试完成" << std::endl;
    return 0;
}
#endif