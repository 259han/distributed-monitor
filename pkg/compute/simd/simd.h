#ifndef SIMD_H
#define SIMD_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdlib.h>

// SIMD加速的向量操作

// 向量加法
void simd_add_f32(const float* a, const float* b, float* result, size_t len);

// 向量减法
void simd_sub_f32(const float* a, const float* b, float* result, size_t len);

// 向量乘法
void simd_mul_f32(const float* a, const float* b, float* result, size_t len);

// 向量点积
float simd_dot_product_f32(const float* a, const float* b, size_t len);

// 向量求和
float simd_sum_f32(const float* a, size_t len);

// 向量最大值
float simd_max_f32(const float* a, size_t len);

// 向量最小值
float simd_min_f32(const float* a, size_t len);

// 检测CPU是否支持SIMD指令集
int simd_check_support();

#ifdef __cplusplus
}
#endif

#endif // SIMD_H
