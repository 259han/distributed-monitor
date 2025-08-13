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
	"github.com/han-fei/monitor/broker/internal/storage"
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
func NewServer(config *Config, storage *storage.RedisStorage) (*Server, error) {
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

	// 创建FSM，并传入Redis存储
	fsm := NewFSM(storage)

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
	log.Printf("正在启动Raft服务器 (节点ID: %s, 地址: %s)", s.config.NodeID, s.config.NodeAddr)

	// 创建Raft配置
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(s.config.NodeID)
	raftConfig.HeartbeatTimeout = s.config.HeartbeatTimeout
	raftConfig.ElectionTimeout = s.config.ElectionTimeout
	raftConfig.CommitTimeout = s.config.CommitTimeout

	// 创建传输层
	log.Printf("正在创建Raft传输层...")
	addr, err := net.ResolveTCPAddr("tcp", s.config.NodeAddr)
	if err != nil {
		log.Printf("解析TCP地址失败: %v", err)
		return err
	}
	transport, err := raft.NewTCPTransport(s.config.NodeAddr, addr, s.config.MaxPool, 10*time.Second, os.Stderr)
	if err != nil {
		log.Printf("创建TCP传输层失败: %v", err)
		return err
	}
	log.Printf("Raft传输层创建成功")

	// 创建日志存储
	log.Printf("正在创建日志存储...")
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(s.config.LogDir, "raft.db"))
	if err != nil {
		log.Printf("创建日志存储失败: %v", err)
		return err
	}

	// 创建稳定存储
	log.Printf("正在创建稳定存储...")
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(s.config.DataDir, "stable.db"))
	if err != nil {
		log.Printf("创建稳定存储失败: %v", err)
		return err
	}

	// 创建快照存储
	log.Printf("正在创建快照存储...")
	snapshotStore, err := raft.NewFileSnapshotStore(s.config.SnapshotDir, 3, os.Stderr)
	if err != nil {
		log.Printf("创建快照存储失败: %v", err)
		return err
	}

	// 创建Raft实例
	log.Printf("正在创建Raft实例...")
	r, err := raft.NewRaft(raftConfig, s.fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		log.Printf("创建Raft实例失败: %v", err)
		return err
	}
	s.raft = r
	log.Printf("Raft实例创建成功")

	// 如果是单节点，直接启动为领导者
	log.Printf("正在引导Raft集群...")
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(s.config.NodeID),
				Address: transport.LocalAddr(),
			},
		},
	}
	s.raft.BootstrapCluster(configuration)
	log.Printf("Raft集群引导完成")

	// 启动gRPC服务器
	log.Printf("正在启动Raft gRPC服务器...")
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
		log.Printf("监听gRPC端口失败: %v", err)
		return err
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterRaftServiceServer(s.grpcServer, s)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC服务器启动失败: %v", err)
		}
	}()

	log.Printf("Raft gRPC服务器启动成功，监听端口: %d", s.config.GRPCPort)
	return nil
}

// AddNode 添加节点
func (s *Server) AddNode(nodeID, address string) error {
	// 检查是否是领导者
	if !s.IsLeader() {
		return fmt.Errorf("only leader can add nodes")
	}

	// 验证节点地址格式
	if !s.isValidNodeAddress(address) {
		return fmt.Errorf("invalid node address: %s", address)
	}

	// 添加投票节点
	addPeerFuture := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(address), 0, 0)
	if err := addPeerFuture.Error(); err != nil {
		return fmt.Errorf("failed to add node %s: %v", nodeID, err)
	}

	log.Printf("成功添加节点: %s (%s)", nodeID, address)
	return nil
}

// RemoveNode 移除节点
func (s *Server) RemoveNode(nodeID string) error {
	// 检查是否是领导者
	if !s.IsLeader() {
		return fmt.Errorf("only leader can remove nodes")
	}

	// 不能移除自己
	if nodeID == s.config.NodeID {
		return fmt.Errorf("cannot remove self")
	}

	// 移除节点
	removePeerFuture := s.raft.RemoveServer(raft.ServerID(nodeID), 0, 0)
	if err := removePeerFuture.Error(); err != nil {
		return fmt.Errorf("failed to remove node %s: %v", nodeID, err)
	}

	log.Printf("成功移除节点: %s", nodeID)
	return nil
}

// Apply 应用命令
func (s *Server) Apply(cmd []byte, timeout time.Duration) (interface{}, error) {
	// 检查是否是领导者
	if !s.IsLeader() {
		return nil, fmt.Errorf("only leader can apply commands")
	}

	// 创建Raft日志条目
	future := s.raft.Apply(cmd, timeout)
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to apply command: %v", err)
	}

	return future.Response(), nil
}

// State 获取当前状态
func (s *Server) State() raft.RaftState {
	return s.raft.State()
}

// Leader 获取当前领导者ID
func (s *Server) Leader() string {
	if s.raft == nil {
		return ""
	}
	return string(s.raft.Leader())
}

// Term 获取当前任期
func (s *Server) Term() uint64 {
	if s.raft == nil {
		return 0
	}
	termStr := s.raft.Stats()["term"]
	return s.parseTerm(termStr)
}

// LastLogIndex 获取最后日志索引
func (s *Server) LastLogIndex() uint64 {
	if s.raft == nil {
		return 0
	}
	return s.raft.LastIndex()
}

// IsLeader 是否是领导者
func (s *Server) IsLeader() bool {
	if s.raft == nil {
		return false
	}
	return s.raft.State() == raft.Leader
}

// GetStats 获取统计信息
func (s *Server) GetStats() map[string]string {
	if s.raft == nil {
		return make(map[string]string)
	}
	return s.raft.Stats()
}

// parseTerm 解析任期字符串
func (s *Server) parseTerm(termStr string) uint64 {
	var term uint64
	fmt.Sscanf(termStr, "%d", &term)
	return term
}

// isValidNodeAddress 验证节点地址格式
func (s *Server) isValidNodeAddress(address string) bool {
	// 简单的地址格式验证
	_, err := net.ResolveTCPAddr("tcp", address)
	return err == nil
}

// applyLogEntry 应用日志条目
func (s *Server) applyLogEntry(entry *pb.LogEntry) error {
	// 创建Raft日志条目
	logEntry := raft.Log{
		Type:  raft.LogCommand,
		Data:  entry.Data,
		Index: entry.Index,
		Term:  entry.Term,
	}

	// 应用到状态机
	future := s.raft.Apply(logEntry.Data, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply log entry: %v", err)
	}

	return nil
}

// GetClusterConfig 获取集群配置
func (s *Server) GetClusterConfig() (raft.Configuration, error) {
	future := s.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return raft.Configuration{}, err
	}
	config := future.Configuration()
	return raft.Configuration{
		Servers: config.Servers,
	}, nil
}

// TransferLeadership 转移领导权
func (s *Server) TransferLeadership(nodeID string) error {
	if !s.IsLeader() {
		return fmt.Errorf("only leader can transfer leadership")
	}

	future := s.raft.LeadershipTransferToServer(raft.ServerID(nodeID), raft.ServerAddress(nodeID))
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to transfer leadership to %s: %v", nodeID, err)
	}

	log.Printf("领导权已转移至节点: %s", nodeID)
	return nil
}

// CreateSnapshot 创建快照
func (s *Server) CreateSnapshot() error {
	if !s.IsLeader() {
		return fmt.Errorf("only leader can create snapshot")
	}

	future := s.raft.Snapshot()
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to create snapshot: %v", err)
	}

	log.Printf("快照创建成功")
	return nil
}

// Barrier 阻塞直到所有日志条目提交
func (s *Server) Barrier() error {
	future := s.raft.Barrier(0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("barrier failed: %v", err)
	}
	return nil
}

// VerifyLeader 验证领导者身份
func (s *Server) VerifyLeader() error {
	future := s.raft.VerifyLeader()
	if err := future.Error(); err != nil {
		return fmt.Errorf("leader verification failed: %v", err)
	}
	return nil
}

// AppendEntries 实现RaftService.AppendEntries
func (s *Server) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	// 获取当前Raft状态
	currentState := s.raft.State()
	currentTerm := s.raft.Stats()["term"]

	// 如果不是领导者，返回重定向
	if currentState != raft.Leader {
		return &pb.AppendEntriesResponse{
			Term:    s.parseTerm(currentTerm),
			Success: false,
		}, nil
	}

	// 验证任期
	if req.Term < s.parseTerm(currentTerm) {
		return &pb.AppendEntriesResponse{
			Term:    s.parseTerm(currentTerm),
			Success: false,
		}, nil
	}

	// 如果任期相同且是跟随者，接受追加日志
	if req.Term == s.parseTerm(currentTerm) && currentState == raft.Follower {
		// 应用日志条目
		for _, entry := range req.Entries {
			if err := s.applyLogEntry(entry); err != nil {
				return &pb.AppendEntriesResponse{
					Term:    s.parseTerm(currentTerm),
					Success: false,
				}, err
			}
		}

		// 更新提交索引
		if req.LeaderCommit > s.raft.CommitIndex() {
			s.raft.Barrier(0) // 简化处理
		}

		return &pb.AppendEntriesResponse{
			Term:    s.parseTerm(currentTerm),
			Success: true,
		}, nil
	}

	// 默认拒绝
	return &pb.AppendEntriesResponse{
		Term:    s.parseTerm(currentTerm),
		Success: false,
	}, nil
}

// RequestVote 实现RaftService.RequestVote
func (s *Server) RequestVote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error) {
	// 获取当前Raft状态
	currentTerm := s.raft.Stats()["term"]
	currentTermUint := s.parseTerm(currentTerm)

	// 如果候选人的任期小于当前任期，拒绝投票
	if req.Term < currentTermUint {
		return &pb.VoteResponse{
			Term:        currentTermUint,
			VoteGranted: false,
		}, nil
	}

	// 如果候选人的任期大于当前任期，更新任期
	if req.Term > currentTermUint {
		// 这里应该更新任期，但Raft库内部会处理
	}

	// 检查是否已经投票给其他候选人
	votedFor := s.raft.Stats()["voted_for"]
	if votedFor != "" && votedFor != req.CandidateId {
		return &pb.VoteResponse{
			Term:        s.parseTerm(currentTerm),
			VoteGranted: false,
		}, nil
	}

	// 检查候选人的日志是否至少和自己的日志一样新
	lastLogIndex := s.raft.LastIndex()
	if req.LastLogIndex < lastLogIndex {
		return &pb.VoteResponse{
			Term:        s.parseTerm(currentTerm),
			VoteGranted: false,
		}, nil
	}

	// 授予投票
	return &pb.VoteResponse{
		Term:        s.parseTerm(currentTerm),
		VoteGranted: true,
	}, nil
}
