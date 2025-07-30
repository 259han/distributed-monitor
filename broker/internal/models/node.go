package models

import (
	"time"
)

// NodeState 节点状态
type NodeState int

const (
	// NodeStateFollower 跟随者状态
	NodeStateFollower NodeState = iota
	// NodeStateCandidate 候选者状态
	NodeStateCandidate
	// NodeStateLeader 领导者状态
	NodeStateLeader
)

// String 返回节点状态的字符串表示
func (s NodeState) String() string {
	switch s {
	case NodeStateFollower:
		return "Follower"
	case NodeStateCandidate:
		return "Candidate"
	case NodeStateLeader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// Node 节点信息
type Node struct {
	ID            string    `json:"id"`             // 节点ID
	Address       string    `json:"address"`        // 节点地址
	State         NodeState `json:"state"`          // 节点状态
	Term          uint64    `json:"term"`           // 当前任期
	VotedFor      string    `json:"voted_for"`      // 投票给谁
	LeaderID      string    `json:"leader_id"`      // 领导者ID
	LastLogIndex  uint64    `json:"last_log_idx"`   // 最后日志索引
	LastLogTerm   uint64    `json:"last_log_term"`  // 最后日志任期
	CommitIndex   uint64    `json:"commit_idx"`     // 已提交索引
	LastApplied   uint64    `json:"last_applied"`   // 已应用索引
	JoinTime      time.Time `json:"join_time"`      // 加入时间
	LastHeartbeat time.Time `json:"last_heartbeat"` // 最后心跳时间
}

// NewNode 创建新节点
func NewNode(id, address string) *Node {
	return &Node{
		ID:            id,
		Address:       address,
		State:         NodeStateFollower,
		Term:          0,
		VotedFor:      "",
		LeaderID:      "",
		LastLogIndex:  0,
		LastLogTerm:   0,
		CommitIndex:   0,
		LastApplied:   0,
		JoinTime:      time.Now(),
		LastHeartbeat: time.Now(),
	}
}

// NodeStatus 节点状态信息
type NodeStatus struct {
	ID            string `json:"id"`             // 节点ID
	Address       string `json:"address"`        // 节点地址
	State         string `json:"state"`          // 节点状态
	Term          uint64 `json:"term"`           // 当前任期
	LeaderID      string `json:"leader_id"`      // 领导者ID
	LastLogIndex  uint64 `json:"last_log_idx"`   // 最后日志索引
	CommitIndex   uint64 `json:"commit_idx"`     // 已提交索引
	LastApplied   uint64 `json:"last_applied"`   // 已应用索引
	Uptime        string `json:"uptime"`         // 运行时间
	LastHeartbeat string `json:"last_heartbeat"` // 最后心跳时间
}

// ToStatus 转换为状态信息
func (n *Node) ToStatus() NodeStatus {
	return NodeStatus{
		ID:            n.ID,
		Address:       n.Address,
		State:         n.State.String(),
		Term:          n.Term,
		LeaderID:      n.LeaderID,
		LastLogIndex:  n.LastLogIndex,
		CommitIndex:   n.CommitIndex,
		LastApplied:   n.LastApplied,
		Uptime:        time.Since(n.JoinTime).String(),
		LastHeartbeat: n.LastHeartbeat.Format(time.RFC3339),
	}
}

// Cluster 集群信息
type Cluster struct {
	Nodes       map[string]*Node `json:"nodes"`        // 节点列表
	LeaderID    string           `json:"leader_id"`    // 领导者ID
	Term        uint64           `json:"term"`         // 当前任期
	CommitIndex uint64           `json:"commit_index"` // 已提交索引
}

// NewCluster 创建新集群
func NewCluster() *Cluster {
	return &Cluster{
		Nodes:       make(map[string]*Node),
		LeaderID:    "",
		Term:        0,
		CommitIndex: 0,
	}
}

// AddNode 添加节点
func (c *Cluster) AddNode(node *Node) {
	c.Nodes[node.ID] = node
}

// RemoveNode 移除节点
func (c *Cluster) RemoveNode(id string) {
	delete(c.Nodes, id)
}

// GetNode 获取节点
func (c *Cluster) GetNode(id string) *Node {
	return c.Nodes[id]
}

// GetLeader 获取领导者节点
func (c *Cluster) GetLeader() *Node {
	if c.LeaderID == "" {
		return nil
	}
	return c.Nodes[c.LeaderID]
}

// GetNodeCount 获取节点数量
func (c *Cluster) GetNodeCount() int {
	return len(c.Nodes)
}

// GetQuorum 获取法定人数
func (c *Cluster) GetQuorum() int {
	return len(c.Nodes)/2 + 1
}
