package gorilla

// #cgo CXXFLAGS: -std=c++11 -O3
// #include <stdlib.h>
// #include "gorilla.h"
import "C"
import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"github.com/han-fei/monitor/pkg/compression"
)

// GorillaCompressor 实现了Gorilla压缩算法
type GorillaCompressor struct {
	compressor unsafe.Pointer
	mu         sync.Mutex
}

// NewGorillaCompressor 创建一个新的Gorilla压缩器
func NewGorillaCompressor() *GorillaCompressor {
	c := &GorillaCompressor{
		compressor: C.gorilla_new_compressor(),
	}
	runtime.SetFinalizer(c, (*GorillaCompressor).Close)
	return c
}

// Compress 压缩时间序列数据
func (g *GorillaCompressor) Compress(points []compression.TimeSeriesPoint) ([]byte, error) {
	if len(points) == 0 {
		return nil, errors.New("empty data points")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// 转换数据点为C结构
	cPoints := C.malloc(C.size_t(len(points) * int(unsafe.Sizeof(C.gorilla_point_t{}))))
	defer C.free(cPoints)

	// 填充C数据点
	for i, p := range points {
		cPoint := (*C.gorilla_point_t)(unsafe.Pointer(uintptr(cPoints) + uintptr(i)*unsafe.Sizeof(C.gorilla_point_t{})))
		cPoint.timestamp = C.int64_t(p.Timestamp)
		cPoint.value = C.double(p.Value)
	}

	// 执行压缩
	result := C.gorilla_compress(g.compressor, (*C.gorilla_point_t)(cPoints), C.size_t(len(points)))
	defer C.gorilla_free_result(&result)

	if result.status != 0 {
		return nil, errors.New("compression failed")
	}

	// 复制结果
	compressed := C.GoBytes(result.data, C.int(result.size))
	return compressed, nil
}

// Decompress 解压缩时间序列数据
func (g *GorillaCompressor) Decompress(data []byte) ([]compression.TimeSeriesPoint, error) {
	if len(data) == 0 {
		return nil, errors.New("empty compressed data")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// 执行解压缩
	result := C.gorilla_decompress(g.compressor, unsafe.Pointer(&data[0]), C.size_t(len(data)))
	defer C.gorilla_free_result(&result)

	if result.status != 0 {
		return nil, errors.New("decompression failed")
	}

	// 转换结果
	count := int(result.size)
	points := make([]compression.TimeSeriesPoint, count)

	// 安全地访问C内存
	cPoints := (*[1 << 20]C.gorilla_point_t)(unsafe.Pointer(result.data))[:count:count]
	for i := 0; i < count; i++ {
		points[i] = compression.TimeSeriesPoint{
			Timestamp: int64(cPoints[i].timestamp),
			Value:     float64(cPoints[i].value),
		}
	}

	return points, nil
}

// Close 关闭压缩器并释放资源
func (g *GorillaCompressor) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.compressor != nil {
		C.gorilla_free_compressor(g.compressor)
		g.compressor = nil
	}
	return nil
}

// 确保GorillaCompressor实现了Compressor接口
var _ compression.Compressor = (*GorillaCompressor)(nil)
