package quic

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"

	pb "github.com/han-fei/monitor/proto"
	"github.com/quic-go/quic-go"
	"google.golang.org/protobuf/proto"
)

const (
	// 协议版本
	ProtocolVersion = 1

	// 消息头大小 (版本1字节 + 类型1字节 + 长度4字节 + 时间戳8字节 + ID长度1字节)
	MessageHeaderSize = 15

	// 最大消息ID长度
	MaxMessageIDLength = 255
)

// MessageHeader 消息头
type MessageHeader struct {
	Version   uint8       `json:"version"`
	Type      MessageType `json:"type"`
	Length    uint32      `json:"length"`
	Timestamp int64       `json:"timestamp"`
	IDLength  uint8       `json:"id_length"`
	ID        string      `json:"id"`
}

// ProtocolHandler 协议处理器
type ProtocolHandler struct {
	maxMessageSize int
}

// NewProtocolHandler 创建协议处理器
func NewProtocolHandler(maxMessageSize int) *ProtocolHandler {
	return &ProtocolHandler{
		maxMessageSize: maxMessageSize,
	}
}

// WriteMessage 写入消息到流
func (p *ProtocolHandler) WriteMessage(stream *quic.Stream, msg *Message) error {
	// 序列化消息数据
	data := msg.Data
	if data == nil {
		data = []byte{}
	}

	// 检查消息大小
	totalSize := MessageHeaderSize + len(msg.ID) + len(data)
	if totalSize > p.maxMessageSize {
		return fmt.Errorf("消息大小超过限制: %d > %d", totalSize, p.maxMessageSize)
	}

	// 构建二进制数据
	buf := make([]byte, 0, MessageHeaderSize+len(msg.ID)+len(data))

	// 写入版本
	buf = append(buf, ProtocolVersion)

	// 写入消息类型
	buf = append(buf, uint8(msg.Type))

	// 写入数据长度
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(data)))
	buf = append(buf, lengthBytes...)

	// 写入时间戳
	timestampBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timestampBytes, uint64(msg.Timestamp))
	buf = append(buf, timestampBytes...)

	// 写入ID长度
	buf = append(buf, uint8(len(msg.ID)))

	// 写入ID
	if len(msg.ID) > 0 {
		buf = append(buf, []byte(msg.ID)...)
	}

	// 写入数据
	if len(data) > 0 {
		buf = append(buf, data...)
	}

	// 一次性写入所有数据
	if _, err := stream.Write(buf); err != nil {
		return fmt.Errorf("写入消息失败: %v", err)
	}

	return nil
}

// ReadMessage 从流读取消息
func (p *ProtocolHandler) ReadMessage(stream *quic.Stream) (*Message, error) {
	// 先读取固定大小的头部
	headerBuf := make([]byte, MessageHeaderSize)
	if _, err := io.ReadFull(stream, headerBuf); err != nil {
		return nil, fmt.Errorf("读取消息头失败: %v", err)
	}

	// 解析头部
	version := headerBuf[0]
	if version != ProtocolVersion {
		return nil, fmt.Errorf("不支持的协议版本: %d", version)
	}

	msgType := MessageType(headerBuf[1])
	length := binary.BigEndian.Uint32(headerBuf[2:6])
	timestamp := int64(binary.BigEndian.Uint64(headerBuf[6:14]))
	idLength := headerBuf[14]

	// 检查数据长度
	if int(length) > p.maxMessageSize {
		return nil, fmt.Errorf("消息大小超过限制: %d > %d", length, p.maxMessageSize)
	}

	// 读取ID
	var id string
	if idLength > 0 {
		idBytes := make([]byte, idLength)
		if _, err := io.ReadFull(stream, idBytes); err != nil {
			return nil, fmt.Errorf("读取ID失败: %v", err)
		}
		id = string(idBytes)
	}

	// 读取数据
	var data []byte
	if length > 0 {
		data = make([]byte, length)
		if _, err := io.ReadFull(stream, data); err != nil {
			return nil, fmt.Errorf("读取数据失败: %v", err)
		}
	}

	return &Message{
		Type:      msgType,
		ID:        id,
		Timestamp: timestamp,
		Data:      data,
	}, nil
}

// EncodeMetrics 编码指标数据
func (p *ProtocolHandler) EncodeMetrics(metrics *pb.MetricsData) ([]byte, error) {
	return proto.Marshal(metrics)
}

// DecodeMetrics 解码指标数据
func (p *ProtocolHandler) DecodeMetrics(data []byte) (*pb.MetricsData, error) {
	metrics := &pb.MetricsData{}
	if err := proto.Unmarshal(data, metrics); err != nil {
		return nil, fmt.Errorf("解码指标数据失败: %v", err)
	}
	return metrics, nil
}

// EncodeSubscription 编码订阅请求
func (p *ProtocolHandler) EncodeSubscription(sub *Subscription) ([]byte, error) {
	return json.Marshal(sub)
}

// DecodeSubscription 解码订阅请求
func (p *ProtocolHandler) DecodeSubscription(data []byte) (*Subscription, error) {
	sub := &Subscription{}
	if err := json.Unmarshal(data, sub); err != nil {
		return nil, fmt.Errorf("解码订阅请求失败: %v", err)
	}
	return sub, nil
}

// CreateMessage 创建消息
func (p *ProtocolHandler) CreateMessage(msgType MessageType, id string, data []byte) *Message {
	return &Message{
		Type:      msgType,
		ID:        id,
		Timestamp: time.Now().UnixNano(),
		Data:      data,
	}
}

// CreateMetricsMessage 创建指标数据消息
func (p *ProtocolHandler) CreateMetricsMessage(id string, metrics *pb.MetricsData) (*Message, error) {
	data, err := p.EncodeMetrics(metrics)
	if err != nil {
		return nil, err
	}

	return p.CreateMessage(MessageTypeMetrics, id, data), nil
}

// CreateHeartbeatMessage 创建心跳消息
func (p *ProtocolHandler) CreateHeartbeatMessage(hostID string) *Message {
	return p.CreateMessage(MessageTypeHeartbeat, hostID, []byte(hostID))
}

// CreateSubscribeMessage 创建订阅消息
func (p *ProtocolHandler) CreateSubscribeMessage(sub *Subscription) (*Message, error) {
	data, err := p.EncodeSubscription(sub)
	if err != nil {
		return nil, err
	}

	return p.CreateMessage(MessageTypeSubscribe, sub.HostID, data), nil
}

// CreateErrorMessage 创建错误消息
func (p *ProtocolHandler) CreateErrorMessage(id string, errMsg string) *Message {
	return p.CreateMessage(MessageTypeError, id, []byte(errMsg))
}

// CreateAckMessage 创建确认消息
func (p *ProtocolHandler) CreateAckMessage(id string) *Message {
	return p.CreateMessage(MessageTypeAck, id, []byte("ok"))
}
