#ifndef GORILLA_H
#define GORILLA_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdlib.h>

// 时间序列数据点结构
typedef struct {
    int64_t timestamp;  // Unix时间戳
    double value;       // 数据点的值
} gorilla_point_t;

// 压缩结果结构
typedef struct {
    void* data;        // 压缩后的数据
    size_t size;       // 数据大小（字节）
    int status;        // 状态码：0=成功，非0=失败
} gorilla_result_t;

// 创建一个新的Gorilla压缩器
void* gorilla_new_compressor();

// 释放压缩器资源
void gorilla_free_compressor(void* compressor);

// 压缩时间序列数据
gorilla_result_t gorilla_compress(void* compressor, const gorilla_point_t* points, size_t count);

// 解压缩时间序列数据
gorilla_result_t gorilla_decompress(void* compressor, const void* data, size_t size);

// 释放结果资源
void gorilla_free_result(gorilla_result_t* result);

#ifdef __cplusplus
}
#endif

#endif // GORILLA_H
