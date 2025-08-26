#include "simd.h"
#include <cmath>
#include <algorithm>
#include <limits>

// 检查是否支持SIMD指令集
#if defined(__x86_64__) || defined(_M_X64) || defined(__i386__) || defined(_M_IX86)
    #include <immintrin.h>
    #define HAS_SSE2 1
    #if defined(__AVX__) || defined(__AVX2__)
        #define HAS_AVX 1
    #else
        #define HAS_AVX 0
    #endif
#else
    #define HAS_SSE2 0
    #define HAS_AVX 0
#endif

// 检测CPU是否支持SIMD指令集
int simd_check_support() {
#if HAS_SSE2
    return 1; // 至少支持SSE2
#else
    return 0; // 不支持SIMD
#endif
}

// 向量加法
void simd_add_f32(const float* a, const float* b, float* result, size_t len) {
#if HAS_AVX
    // 使用AVX指令集（一次处理8个浮点数）
    size_t i = 0;
    for (; i + 7 < len; i += 8) {
        __m256 va = _mm256_loadu_ps(&a[i]);
        __m256 vb = _mm256_loadu_ps(&b[i]);
        __m256 vr = _mm256_add_ps(va, vb);
        _mm256_storeu_ps(&result[i], vr);
    }
    // 处理剩余元素
    for (; i < len; ++i) {
        result[i] = a[i] + b[i];
    }
#elif HAS_SSE2
    // 使用SSE2指令集（一次处理4个浮点数）
    size_t i = 0;
    for (; i + 3 < len; i += 4) {
        __m128 va = _mm_loadu_ps(&a[i]);
        __m128 vb = _mm_loadu_ps(&b[i]);
        __m128 vr = _mm_add_ps(va, vb);
        _mm_storeu_ps(&result[i], vr);
    }
    // 处理剩余元素
    for (; i < len; ++i) {
        result[i] = a[i] + b[i];
    }
#else
    // 标量实现
    for (size_t i = 0; i < len; ++i) {
        result[i] = a[i] + b[i];
    }
#endif
}

// 向量减法
void simd_sub_f32(const float* a, const float* b, float* result, size_t len) {
#if HAS_AVX
    // 使用AVX指令集
    size_t i = 0;
    for (; i + 7 < len; i += 8) {
        __m256 va = _mm256_loadu_ps(&a[i]);
        __m256 vb = _mm256_loadu_ps(&b[i]);
        __m256 vr = _mm256_sub_ps(va, vb);
        _mm256_storeu_ps(&result[i], vr);
    }
    // 处理剩余元素
    for (; i < len; ++i) {
        result[i] = a[i] - b[i];
    }
#elif HAS_SSE2
    // 使用SSE2指令集
    size_t i = 0;
    for (; i + 3 < len; i += 4) {
        __m128 va = _mm_loadu_ps(&a[i]);
        __m128 vb = _mm_loadu_ps(&b[i]);
        __m128 vr = _mm_sub_ps(va, vb);
        _mm_storeu_ps(&result[i], vr);
    }
    // 处理剩余元素
    for (; i < len; ++i) {
        result[i] = a[i] - b[i];
    }
#else
    // 标量实现
    for (size_t i = 0; i < len; ++i) {
        result[i] = a[i] - b[i];
    }
#endif
}

// 向量乘法
void simd_mul_f32(const float* a, const float* b, float* result, size_t len) {
#if HAS_AVX
    // 使用AVX指令集
    size_t i = 0;
    for (; i + 7 < len; i += 8) {
        __m256 va = _mm256_loadu_ps(&a[i]);
        __m256 vb = _mm256_loadu_ps(&b[i]);
        __m256 vr = _mm256_mul_ps(va, vb);
        _mm256_storeu_ps(&result[i], vr);
    }
    // 处理剩余元素
    for (; i < len; ++i) {
        result[i] = a[i] * b[i];
    }
#elif HAS_SSE2
    // 使用SSE2指令集
    size_t i = 0;
    for (; i + 3 < len; i += 4) {
        __m128 va = _mm_loadu_ps(&a[i]);
        __m128 vb = _mm_loadu_ps(&b[i]);
        __m128 vr = _mm_mul_ps(va, vb);
        _mm_storeu_ps(&result[i], vr);
    }
    // 处理剩余元素
    for (; i < len; ++i) {
        result[i] = a[i] * b[i];
    }
#else
    // 标量实现
    for (size_t i = 0; i < len; ++i) {
        result[i] = a[i] * b[i];
    }
#endif
}

// 向量点积
float simd_dot_product_f32(const float* a, const float* b, size_t len) {
    float sum = 0.0f;

#if HAS_AVX
    // 使用AVX指令集
    __m256 vsum = _mm256_setzero_ps();
    size_t i = 0;
    for (; i + 7 < len; i += 8) {
        __m256 va = _mm256_loadu_ps(&a[i]);
        __m256 vb = _mm256_loadu_ps(&b[i]);
        __m256 vmul = _mm256_mul_ps(va, vb);
        vsum = _mm256_add_ps(vsum, vmul);
    }
    
    // 水平求和
    float temp[8];
    _mm256_storeu_ps(temp, vsum);
    sum = temp[0] + temp[1] + temp[2] + temp[3] + temp[4] + temp[5] + temp[6] + temp[7];
    
    // 处理剩余元素
    for (; i < len; ++i) {
        sum += a[i] * b[i];
    }
#elif HAS_SSE2
    // 使用SSE2指令集
    __m128 vsum = _mm_setzero_ps();
    size_t i = 0;
    for (; i + 3 < len; i += 4) {
        __m128 va = _mm_loadu_ps(&a[i]);
        __m128 vb = _mm_loadu_ps(&b[i]);
        __m128 vmul = _mm_mul_ps(va, vb);
        vsum = _mm_add_ps(vsum, vmul);
    }
    
    // 水平求和
    float temp[4];
    _mm_storeu_ps(temp, vsum);
    sum = temp[0] + temp[1] + temp[2] + temp[3];
    
    // 处理剩余元素
    for (; i < len; ++i) {
        sum += a[i] * b[i];
    }
#else
    // 标量实现
    for (size_t i = 0; i < len; ++i) {
        sum += a[i] * b[i];
    }
#endif

    return sum;
}

// 向量求和
float simd_sum_f32(const float* a, size_t len) {
    float sum = 0.0f;

#if HAS_AVX
    // 使用AVX指令集
    __m256 vsum = _mm256_setzero_ps();
    size_t i = 0;
    for (; i + 7 < len; i += 8) {
        __m256 va = _mm256_loadu_ps(&a[i]);
        vsum = _mm256_add_ps(vsum, va);
    }
    
    // 水平求和
    float temp[8];
    _mm256_storeu_ps(temp, vsum);
    sum = temp[0] + temp[1] + temp[2] + temp[3] + temp[4] + temp[5] + temp[6] + temp[7];
    
    // 处理剩余元素
    for (; i < len; ++i) {
        sum += a[i];
    }
#elif HAS_SSE2
    // 使用SSE2指令集
    __m128 vsum = _mm_setzero_ps();
    size_t i = 0;
    for (; i + 3 < len; i += 4) {
        __m128 va = _mm_loadu_ps(&a[i]);
        vsum = _mm_add_ps(vsum, va);
    }
    
    // 水平求和
    float temp[4];
    _mm_storeu_ps(temp, vsum);
    sum = temp[0] + temp[1] + temp[2] + temp[3];
    
    // 处理剩余元素
    for (; i < len; ++i) {
        sum += a[i];
    }
#else
    // 标量实现
    for (size_t i = 0; i < len; ++i) {
        sum += a[i];
    }
#endif

    return sum;
}

// 向量最大值
float simd_max_f32(const float* a, size_t len) {
    if (len == 0) return -std::numeric_limits<float>::infinity();
    
    float max_val = a[0];

#if HAS_AVX
    // 使用AVX指令集
    __m256 vmax = _mm256_set1_ps(a[0]);
    size_t i = 0;
    for (; i + 7 < len; i += 8) {
        __m256 va = _mm256_loadu_ps(&a[i]);
        vmax = _mm256_max_ps(vmax, va);
    }
    
    // 提取最大值
    float temp[8];
    _mm256_storeu_ps(temp, vmax);
    max_val = std::max({temp[0], temp[1], temp[2], temp[3], temp[4], temp[5], temp[6], temp[7]});
    
    // 处理剩余元素
    for (; i < len; ++i) {
        max_val = std::max(max_val, a[i]);
    }
#elif HAS_SSE2
    // 使用SSE2指令集
    __m128 vmax = _mm_set1_ps(a[0]);
    size_t i = 0;
    for (; i + 3 < len; i += 4) {
        __m128 va = _mm_loadu_ps(&a[i]);
        vmax = _mm_max_ps(vmax, va);
    }
    
    // 提取最大值
    float temp[4];
    _mm_storeu_ps(temp, vmax);
    max_val = std::max({temp[0], temp[1], temp[2], temp[3]});
    
    // 处理剩余元素
    for (; i < len; ++i) {
        max_val = std::max(max_val, a[i]);
    }
#else
    // 标量实现
    for (size_t i = 1; i < len; ++i) {
        max_val = std::max(max_val, a[i]);
    }
#endif

    return max_val;
}

// 向量最小值
float simd_min_f32(const float* a, size_t len) {
    if (len == 0) return std::numeric_limits<float>::infinity();
    
    float min_val = a[0];

#if HAS_AVX
    // 使用AVX指令集
    __m256 vmin = _mm256_set1_ps(a[0]);
    size_t i = 0;
    for (; i + 7 < len; i += 8) {
        __m256 va = _mm256_loadu_ps(&a[i]);
        vmin = _mm256_min_ps(vmin, va);
    }
    
    // 提取最小值
    float temp[8];
    _mm256_storeu_ps(temp, vmin);
    min_val = std::min({temp[0], temp[1], temp[2], temp[3], temp[4], temp[5], temp[6], temp[7]});
    
    // 处理剩余元素
    for (; i < len; ++i) {
        min_val = std::min(min_val, a[i]);
    }
#elif HAS_SSE2
    // 使用SSE2指令集
    __m128 vmin = _mm_set1_ps(a[0]);
    size_t i = 0;
    for (; i + 3 < len; i += 4) {
        __m128 va = _mm_loadu_ps(&a[i]);
        vmin = _mm_min_ps(vmin, va);
    }
    
    // 提取最小值
    float temp[4];
    _mm_storeu_ps(temp, vmin);
    min_val = std::min({temp[0], temp[1], temp[2], temp[3]});
    
    // 处理剩余元素
    for (; i < len; ++i) {
        min_val = std::min(min_val, a[i]);
    }
#else
    // 标量实现
    for (size_t i = 1; i < len; ++i) {
        min_val = std::min(min_val, a[i]);
    }
#endif

    return min_val;
}
