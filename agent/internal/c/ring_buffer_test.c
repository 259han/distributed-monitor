#include "ring_buffer.h"

// 测试主函数
#ifdef COMPILE_TEST
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main() {
    printf("开始ring buffer测试...\n");
    
    // 创建ring buffer
    ring_buffer_t *rb = ring_buffer_create(1024, 1);
    if (!rb) {
        printf("✗ 创建ring buffer失败\n");
        return 1;
    }
    printf("✓ 创建ring buffer成功\n");
    
    // 测试基本操作
    char *test_data[10];
    for (int i = 0; i < 10; i++) {
        test_data[i] = malloc(32);
        sprintf(test_data[i], "测试数据%d", i);
    }
    
    // 添加数据
    for (int i = 0; i < 5; i++) {
        if (ring_buffer_push(rb, test_data[i]) == 0) {
            printf("✓ 添加数据%d成功\n", i);
        } else {
            printf("✗ 添加数据%d失败\n", i);
        }
    }
    
    // 获取数据
    void *data;
    for (int i = 0; i < 5; i++) {
        if (ring_buffer_pop(rb, &data) == 0) {
            printf("✓ 获取数据: %s\n", (char*)data);
            // 注意：ring buffer拥有数据的所有权，不应该在这里释放
        } else {
            printf("✗ 获取数据失败\n");
        }
    }
    
    // 测试批量操作
    char *batch_data[3];
    for (int i = 0; i < 3; i++) {
        batch_data[i] = malloc(32);
        sprintf(batch_data[i], "批量数据%d", i);
    }
    
    if (ring_buffer_push_batch(rb, (void**)batch_data, 3) == 0) {
        printf("✓ 批量添加成功\n");
    } else {
        printf("✗ 批量添加失败\n");
    }
    
    // 测试自动调整
    printf("当前容量: %zu\n", ring_buffer_capacity(rb));
    printf("当前大小: %zu\n", ring_buffer_size(rb));
    
    // 清理
    ring_buffer_destroy(rb); // ring buffer会自动清理其中的数据
    // 只清理未添加到ring buffer的数据
    for (int i = 5; i < 10; i++) {
        free(test_data[i]);
    }
    
    printf("✓ ring buffer测试完成\n");
    return 0;
}
#endif