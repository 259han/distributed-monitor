package raft

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"google.golang.org/grpc"

	"github.com/han-fei/monitor/broker/internal/models"
	pb "github.com/han-fei/monitor/proto"
)

// Server Raft服务器
type Server struct {
	config     *Config
	raft       *raft.Raft
	fsm        *FSM
	grpcServer *grpc.Server
	node       *models.Node
	pb.UnimplementedRaftServiceServer
}

// Config Raft服务器配置
type Config struct {
	NodeID           string
	NodeAddr         string
	DataDir          string
	LogDir           string
	SnapshotDir      string
	GRPCPort         int
	Peers            []string
	HeartbeatTimeout time.Duration
	ElectionTimeout  time.Duration
	CommitTimeout    time.Duration
	MaxPool          int
}

// NewServer 创建新的Raft服务器
func NewServer(config *Config) (*Server, error) {
	// 创建目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(config.SnapshotDir, 0755); err != nil {
		return nil, err
	}

	// 创建节点
	node := models.NewNode(config.NodeID, config.NodeAddr)

	// 创建FSM
	fsm := NewFSM()

	// 创建服务器
	server := &Server{
		config: config,
		fsm:    fsm,
		node:   node,
	}

	return server, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	// 创建Raft配置
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(s.config.NodeID)
	raftConfig.HeartbeatTimeout = s.config.HeartbeatTimeout
	raftConfig.ElectionTimeout = s.config.ElectionTimeout
	raftConfig.CommitTimeout = s.config.CommitTimeout

	// 创建传输层
	addr, err := net.ResolveTCPAddr("tcp", s.config.NodeAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(s.config.NodeAddr, addr, s.config.MaxPool, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// 创建日志存储
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(s.config.LogDir, "raft.db"))
	if err != nil {
		return err
	}

	// 创建稳定存储
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(s.config.DataDir, "stable.db"))
	if err != nil {
		return err
	}

	// 创建快照存储
	snapshotStore, err := raft.NewFileSnapshotStore(s.config.SnapshotDir, 3, os.Stderr)
	if err != nil {
		return err
	}

	// 创建Raft实例
	r, err := raft.NewRaft(raftConfig, s.fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}
	s.raft = r

	// 如果是单节点，直接启动为领导者
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(s.config.NodeID),
				Address: transport.LocalAddr(),
			},
		},
	}
	s.raft.BootstrapCluster(configuration)

	// 启动gRPC服务器
	return s.startGRPCServer()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	// 关闭Raft
	if s.raft != nil {
		future := s.raft.Shutdown()
		if err := future.Error(); err != nil {
			return err
		}
	}

	// 关闭gRPC服务器
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	return nil
}

// startGRPCServer 启动gRPC服务器
func (s *Server) startGRPCServer() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.GRPCPort))
	if err != nil {
		return err
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterRaftServiceServer(s.grpcServer, s)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC服务器启动失败: %v", err)
		}
	}()

	return nil
}

// AddNode 添加节点
func (s *Server) AddNode(nodeID, address string) error {
	addPeerFuture := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(address), 0, 0)
	if err := addPeerFuture.Error(); err != nil {
		return err
	}
	return nil
}

// RemoveNode 移除节点
func (s *Server) RemoveNode(nodeID string) error {
	removePeerFuture := s.raft.RemoveServer(raft.ServerID(nodeID), 0, 0)
	if err := removePeerFuture.Error(); err != nil {
		return err
	}
	return nil
}

// Apply 应用命令
func (s *Server) Apply(cmd []byte, timeout time.Duration) (interface{}, error) {
	future := s.raft.Apply(cmd, timeout)
	if err := future.Error(); err != nil {
		return nil, err
	}
	return future.Response(), nil
}

// State 获取当前状态
func (s *Server) State() raft.RaftState {
	return s.raft.State()
}

// Leader 获取当前领导者ID
func (s *Server) Leader() string {
	return string(s.raft.Leader())
}

// Term 获取当前任期（简化处理，实际应通过Raft状态机获取）
func (s *Server) Term() uint64 {
	return 1 // 简化处理，返回固定值
}

// LastLogIndex 获取最后日志索引
func (s *Server) LastLogIndex() uint64 {
	return s.raft.LastIndex()
}

// IsLeader 是否是领导者
func (s *Server) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

// GetStats 获取统计信息
func (s *Server) GetStats() map[string]string {
	return s.raft.Stats()
}

// AppendEntries 实现RaftService.AppendEntries
func (s *Server) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	// 这里应该实现AppendEntries RPC
	// 简化处理，直接返回成功
	return &pb.AppendEntriesResponse{
		Term:    1, // 简化处理，返回固定值
		Success: true,
	}, nil
}

// RequestVote 实现RaftService.RequestVote
func (s *Server) RequestVote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error) {
	// 这里应该实现RequestVote RPC
	// 简化处理，直接返回成功
	return &pb.VoteResponse{
		Term:        1, // 简化处理，返回固定值
		VoteGranted: true,
	}, nil
}
