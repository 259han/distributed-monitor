#include "gorilla.h"
#include <vector>
#include <cstring>
#include <cmath>
#include <algorithm>

// BitWriter 类，用于位级操作写入
class BitWriter {
private:
    std::vector<uint8_t> buffer_;
    size_t bit_position_ = 0;

public:
    // 写入单个位
    void writeBit(bool bit) {
        size_t byte_pos = bit_position_ / 8;
        size_t bit_offset = bit_position_ % 8;
        
        if (byte_pos >= buffer_.size()) {
            buffer_.push_back(0);
        }
        
        if (bit) {
            buffer_[byte_pos] |= (1 << bit_offset);
        }
        
        bit_position_++;
    }
    
    // 写入多个位
    void writeBits(uint64_t value, int bits) {
        for (int i = 0; i < bits; i++) {
            writeBit((value >> i) & 1);
        }
    }
    
    // 获取缓冲区数据
    const uint8_t* data() const {
        return buffer_.data();
    }
    
    // 获取缓冲区大小（字节）
    size_t size() const {
        return buffer_.size();
    }
};

// BitReader 类，用于位级操作读取
class BitReader {
private:
    const uint8_t* buffer_;
    size_t buffer_size_;
    size_t bit_position_ = 0;
    size_t point_count_ = 0;
    size_t points_read_ = 0;

public:
    BitReader(const void* data, size_t size) 
        : buffer_(static_cast<const uint8_t*>(data)), buffer_size_(size) {
        // 读取数据点数量
        point_count_ = static_cast<size_t>(readBits(32));
    }
    
    // 读取单个位
    bool readBit() {
        if (bit_position_ / 8 >= buffer_size_) {
            return false; // 安全检查
        }
        
        size_t byte_pos = bit_position_ / 8;
        size_t bit_offset = bit_position_ % 8;
        
        bool bit = (buffer_[byte_pos] >> bit_offset) & 1;
        bit_position_++;
        return bit;
    }
    
    // 读取多个位
    uint64_t readBits(int bits) {
        uint64_t value = 0;
        for (int i = 0; i < bits; i++) {
            if (readBit()) {
                value |= (1ULL << i);
            }
        }
        return value;
    }
    
    // 是否还有更多数据
    bool hasMore() {
        points_read_++;
        return points_read_ < point_count_;
    }
};

// Gorilla压缩器实现
class GorillaCompressor {
private:
    std::vector<gorilla_point_t> points_;
    
public:
    // 压缩时间序列数据
    gorilla_result_t compress(const gorilla_point_t* points, size_t count) {
        gorilla_result_t result = {nullptr, 0, 0};
        if (!points || count == 0) {
            result.status = 1;
            return result;
        }
        
        // 保存输入数据
        points_.clear();
        points_.reserve(count);
        for (size_t i = 0; i < count; i++) {
            points_.push_back(points[i]);
        }
        
        // 排序数据点（按时间戳）
        std::sort(points_.begin(), points_.end(), 
            [](const gorilla_point_t& a, const gorilla_point_t& b) {
                return a.timestamp < b.timestamp;
            });
        
        BitWriter writer;
        
        // 写入数据点数量
        writer.writeBits(static_cast<uint64_t>(points_.size()), 32);
        
        // 写入第一个数据点（完整存储）
        if (!points_.empty()) {
            writer.writeBits(static_cast<uint64_t>(points_[0].timestamp), 64);
            writer.writeBits(*reinterpret_cast<uint64_t*>(&points_[0].value), 64);
            
            int64_t prev_timestamp = points_[0].timestamp;
            int64_t prev_delta = 0;
            uint64_t prev_value_bits = *reinterpret_cast<uint64_t*>(&points_[0].value);
            
            // 压缩后续数据点
            for (size_t i = 1; i < points_.size(); i++) {
                // 压缩时间戳（Delta-of-delta编码）
                int64_t delta = points_[i].timestamp - prev_timestamp;
                int64_t delta_of_delta = delta - prev_delta;
                
                if (delta_of_delta == 0) {
                    writer.writeBit(false); // 1位表示无变化
                } else if (delta_of_delta >= -63 && delta_of_delta <= 64) {
                    writer.writeBits(0b10, 2); // 2位控制码
                    writer.writeBits(delta_of_delta + 63, 7); // 7位数据
                } else if (delta_of_delta >= -255 && delta_of_delta <= 256) {
                    writer.writeBits(0b110, 3); // 3位控制码
                    writer.writeBits(delta_of_delta + 255, 9); // 9位数据
                } else if (delta_of_delta >= -2047 && delta_of_delta <= 2048) {
                    writer.writeBits(0b1110, 4); // 4位控制码
                    writer.writeBits(delta_of_delta + 2047, 12); // 12位数据
                } else {
                    writer.writeBits(0b1111, 4); // 4位控制码
                    writer.writeBits(delta_of_delta, 64); // 64位数据
                }
                
                prev_timestamp = points_[i].timestamp;
                prev_delta = delta;
                
                // 压缩数值（XOR差分编码）
                uint64_t value_bits = *reinterpret_cast<uint64_t*>(&points_[i].value);
                uint64_t xor_result = value_bits ^ prev_value_bits;
                
                if (xor_result == 0) {
                    writer.writeBit(false); // 值未变化
                } else {
                    writer.writeBit(true);
                    
                    // 计算前导零和尾随零
                    int leading_zeros = __builtin_clzll(xor_result);
                    int trailing_zeros = __builtin_ctzll(xor_result);
                    int significant_bits = 64 - leading_zeros - trailing_zeros;
                    
                    if (significant_bits <= 5) {
                        // 控制位 + 有效位数 + 有效位
                        writer.writeBit(false);
                        writer.writeBits(significant_bits - 1, 3);
                        writer.writeBits(xor_result >> trailing_zeros, significant_bits);
                    } else {
                        // 控制位 + 前导零 + 有效位数 + 有效位
                        writer.writeBit(true);
                        writer.writeBits(leading_zeros, 6);
                        writer.writeBits(64 - leading_zeros - trailing_zeros, 6);
                        writer.writeBits(xor_result >> trailing_zeros, 64 - leading_zeros - trailing_zeros);
                    }
                }
                
                prev_value_bits = value_bits;
            }
        }
        
        // 分配结果内存
        result.size = writer.size();
        result.data = malloc(result.size);
        if (!result.data) {
            result.status = 2;
            return result;
        }
        
        // 复制压缩数据
        memcpy(result.data, writer.data(), result.size);
        result.status = 0;
        return result;
    }
    
    // 解压缩时间序列数据
    gorilla_result_t decompress(const void* data, size_t size) {
        gorilla_result_t result = {nullptr, 0, 0};
        if (!data || size == 0) {
            result.status = 1;
            return result;
        }
        
        try {
            BitReader reader(data, size);
            std::vector<gorilla_point_t> points;
            
            // 读取第一个数据点
            int64_t timestamp = static_cast<int64_t>(reader.readBits(64));
            double value;
            uint64_t value_bits = reader.readBits(64);
            memcpy(&value, &value_bits, sizeof(double));
            
            points.push_back({timestamp, value});
            
            int64_t prev_timestamp = timestamp;
            int64_t prev_delta = 0;
            uint64_t prev_value_bits = value_bits;
            
            // 解压后续数据点
            while (reader.hasMore()) {
                // 解压时间戳
                int64_t delta_of_delta = 0;
                
                if (!reader.readBit()) {
                    delta_of_delta = 0;
                } else if (!reader.readBit()) {
                    delta_of_delta = static_cast<int64_t>(reader.readBits(7)) - 63;
                } else if (!reader.readBit()) {
                    delta_of_delta = static_cast<int64_t>(reader.readBits(9)) - 255;
                } else if (!reader.readBit()) {
                    delta_of_delta = static_cast<int64_t>(reader.readBits(12)) - 2047;
                } else {
                    delta_of_delta = static_cast<int64_t>(reader.readBits(64));
                }
                
                int64_t delta = prev_delta + delta_of_delta;
                timestamp = prev_timestamp + delta;
                prev_timestamp = timestamp;
                prev_delta = delta;
                
                // 解压数值
                uint64_t value_bits;
                
                if (!reader.readBit()) {
                    value_bits = prev_value_bits;
                } else {
                    if (!reader.readBit()) {
                        // 短格式
                        int significant_bits = static_cast<int>(reader.readBits(3)) + 1;
                        uint64_t xor_bits = reader.readBits(significant_bits);
                        value_bits = prev_value_bits ^ xor_bits;
                    } else {
                        // 长格式
                        int leading_zeros = static_cast<int>(reader.readBits(6));
                        int significant_bits = static_cast<int>(reader.readBits(6));
                        uint64_t xor_bits = reader.readBits(significant_bits);
                        value_bits = prev_value_bits ^ (xor_bits << (64 - leading_zeros - significant_bits));
                    }
                }
                
                memcpy(&value, &value_bits, sizeof(double));
                points.push_back({timestamp, value});
                prev_value_bits = value_bits;
            }
            
            // 分配结果内存
            size_t result_size = points.size() * sizeof(gorilla_point_t);
            gorilla_point_t* result_points = static_cast<gorilla_point_t*>(malloc(result_size));
            if (!result_points) {
                result.status = 2;
                return result;
            }
            
            // 复制解压数据
            memcpy(result_points, points.data(), result_size);
            
            result.data = result_points;
            result.size = points.size();
            result.status = 0;
            return result;
        } catch (...) {
            result.status = 3;
            return result;
        }
    }
};

// C接口实现

extern "C" {
    // 创建一个新的Gorilla压缩器
    void* gorilla_new_compressor() {
        return new GorillaCompressor();
    }

    // 释放压缩器资源
    void gorilla_free_compressor(void* compressor) {
        delete static_cast<GorillaCompressor*>(compressor);
    }

    // 压缩时间序列数据
    gorilla_result_t gorilla_compress(void* compressor, const gorilla_point_t* points, size_t count) {
        return static_cast<GorillaCompressor*>(compressor)->compress(points, count);
    }

    // 解压缩时间序列数据
    gorilla_result_t gorilla_decompress(void* compressor, const void* data, size_t size) {
        return static_cast<GorillaCompressor*>(compressor)->decompress(data, size);
    }

    // 释放结果资源
    void gorilla_free_result(gorilla_result_t* result) {
        if (result && result->data) {
            free(result->data);
            result->data = nullptr;
            result->size = 0;
        }
    }
}
