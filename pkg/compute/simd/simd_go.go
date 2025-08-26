package simd

// #cgo CXXFLAGS: -std=c++11 -O3 -march=native
// #include <stdlib.h>
// #include "simd.h"
import "C"
import (
	"unsafe"
)

// IsSupported 检查当前CPU是否支持SIMD指令集
func IsSupported() bool {
	return C.simd_check_support() != 0
}

// AddFloat32 向量加法
func AddFloat32(a, b []float32) []float32 {
	if len(a) != len(b) {
		return nil
	}
	
	result := make([]float32, len(a))
	if len(a) == 0 {
		return result
	}
	
	C.simd_add_f32(
		(*C.float)(unsafe.Pointer(&a[0])),
		(*C.float)(unsafe.Pointer(&b[0])),
		(*C.float)(unsafe.Pointer(&result[0])),
		C.size_t(len(a)),
	)
	
	return result
}

// SubFloat32 向量减法
func SubFloat32(a, b []float32) []float32 {
	if len(a) != len(b) {
		return nil
	}
	
	result := make([]float32, len(a))
	if len(a) == 0 {
		return result
	}
	
	C.simd_sub_f32(
		(*C.float)(unsafe.Pointer(&a[0])),
		(*C.float)(unsafe.Pointer(&b[0])),
		(*C.float)(unsafe.Pointer(&result[0])),
		C.size_t(len(a)),
	)
	
	return result
}

// MulFloat32 向量乘法
func MulFloat32(a, b []float32) []float32 {
	if len(a) != len(b) {
		return nil
	}
	
	result := make([]float32, len(a))
	if len(a) == 0 {
		return result
	}
	
	C.simd_mul_f32(
		(*C.float)(unsafe.Pointer(&a[0])),
		(*C.float)(unsafe.Pointer(&b[0])),
		(*C.float)(unsafe.Pointer(&result[0])),
		C.size_t(len(a)),
	)
	
	return result
}

// DotProductFloat32 向量点积
func DotProductFloat32(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	
	return float32(C.simd_dot_product_f32(
		(*C.float)(unsafe.Pointer(&a[0])),
		(*C.float)(unsafe.Pointer(&b[0])),
		C.size_t(len(a)),
	))
}

// SumFloat32 向量求和
func SumFloat32(a []float32) float32 {
	if len(a) == 0 {
		return 0
	}
	
	return float32(C.simd_sum_f32(
		(*C.float)(unsafe.Pointer(&a[0])),
		C.size_t(len(a)),
	))
}

// MaxFloat32 向量最大值
func MaxFloat32(a []float32) float32 {
	if len(a) == 0 {
		return 0
	}
	
	return float32(C.simd_max_f32(
		(*C.float)(unsafe.Pointer(&a[0])),
		C.size_t(len(a)),
	))
}

// MinFloat32 向量最小值
func MinFloat32(a []float32) float32 {
	if len(a) == 0 {
		return 0
	}
	
	return float32(C.simd_min_f32(
		(*C.float)(unsafe.Pointer(&a[0])),
		C.size_t(len(a)),
	))
}
