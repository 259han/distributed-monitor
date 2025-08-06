package quic

import (
	"context"
	"log"

	pb "github.com/han-fei/monitor/proto"
)

// DefaultDataHandler 默认数据处理器实现
type DefaultDataHandler struct {
	metricsHandler     func(ctx context.Context, metrics *pb.MetricsData) error
	heartbeatHandler   func(ctx context.Context, hostID string) error
	subscribeHandler   func(ctx context.Context, hostID, streamID string) error
	unsubscribeHandler func(ctx context.Context, hostID, streamID string) error
}

// NewDefaultDataHandler 创建默认数据处理器
func NewDefaultDataHandler() *DefaultDataHandler {
	return &DefaultDataHandler{
		metricsHandler: func(ctx context.Context, metrics *pb.MetricsData) error {
			log.Printf("收到指标数据: HostID=%s, 指标数量=%d",
				metrics.HostId, len(metrics.Metrics))
			return nil
		},
		heartbeatHandler: func(ctx context.Context, hostID string) error {
			log.Printf("收到心跳: HostID=%s", hostID)
			return nil
		},
		subscribeHandler: func(ctx context.Context, hostID, streamID string) error {
			log.Printf("订阅请求: HostID=%s, StreamID=%s", hostID, streamID)
			return nil
		},
		unsubscribeHandler: func(ctx context.Context, hostID, streamID string) error {
			log.Printf("取消订阅: HostID=%s, StreamID=%s", hostID, streamID)
			return nil
		},
	}
}

// SetMetricsHandler 设置指标数据处理器
func (h *DefaultDataHandler) SetMetricsHandler(handler func(ctx context.Context, metrics *pb.MetricsData) error) {
	h.metricsHandler = handler
}

// SetHeartbeatHandler 设置心跳处理器
func (h *DefaultDataHandler) SetHeartbeatHandler(handler func(ctx context.Context, hostID string) error) {
	h.heartbeatHandler = handler
}

// SetSubscribeHandler 设置订阅处理器
func (h *DefaultDataHandler) SetSubscribeHandler(handler func(ctx context.Context, hostID, streamID string) error) {
	h.subscribeHandler = handler
}

// SetUnsubscribeHandler 设置取消订阅处理器
func (h *DefaultDataHandler) SetUnsubscribeHandler(handler func(ctx context.Context, hostID, streamID string) error) {
	h.unsubscribeHandler = handler
}

// HandleMetrics 处理指标数据
func (h *DefaultDataHandler) HandleMetrics(ctx context.Context, metrics *pb.MetricsData) error {
	if h.metricsHandler != nil {
		return h.metricsHandler(ctx, metrics)
	}
	return nil
}

// HandleHeartbeat 处理心跳
func (h *DefaultDataHandler) HandleHeartbeat(ctx context.Context, hostID string) error {
	if h.heartbeatHandler != nil {
		return h.heartbeatHandler(ctx, hostID)
	}
	return nil
}

// HandleSubscribe 处理订阅
func (h *DefaultDataHandler) HandleSubscribe(ctx context.Context, hostID, streamID string) error {
	if h.subscribeHandler != nil {
		return h.subscribeHandler(ctx, hostID, streamID)
	}
	return nil
}

// HandleUnsubscribe 处理取消订阅
func (h *DefaultDataHandler) HandleUnsubscribe(ctx context.Context, hostID, streamID string) error {
	if h.unsubscribeHandler != nil {
		return h.unsubscribeHandler(ctx, hostID, streamID)
	}
	return nil
}
