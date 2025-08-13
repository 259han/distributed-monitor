package utils

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

// CircuitBreaker 熔断器模式
type CircuitBreaker struct {
	maxFailures    int           // 最大失败次数
	resetTimeout  time.Duration // 重置超时时间
	state          string        // 状态：closed, open, half-open
	failures       int           // 失败次数
	lastFailure    time.Time     // 最后失败时间
	mu             sync.RWMutex   // 读写锁
}

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:   maxFailures,
		resetTimeout:  resetTimeout,
		state:         "closed",
		failures:      0,
		lastFailure:   time.Time{},
	}
}

// Execute 执行操作
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.RLock()
	if cb.state == "open" {
		if time.Since(cb.lastFailure) < cb.resetTimeout {
			cb.mu.RUnlock()
			return errors.New("circuit breaker is open")
		}
		// 进入半开状态
		cb.mu.RUnlock()
		cb.mu.Lock()
		cb.state = "half-open"
		cb.mu.Unlock()
	} else {
		cb.mu.RUnlock()
	}

	err := fn()
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		
		if cb.failures >= cb.maxFailures {
			cb.state = "open"
			log.Printf("熔断器打开，失败次数: %d", cb.failures)
		}
		return err
	}
	
	// 成功执行，重置状态
	cb.failures = 0
	cb.state = "closed"
	return nil
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailures 获取失败次数
func (cb *CircuitBreaker) GetFailures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// RateLimiter 速率限制器
type RateLimiter struct {
	rate       int           // 速率（每秒请求数）
	capacity   int           // 容量
	tokens     int           // 当前令牌数
	lastRefill time.Time     // 最后补充时间
	mu         sync.Mutex    // 互斥锁
}

// NewRateLimiter 创建新的速率限制器
func NewRateLimiter(rate int, capacity int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// Allow 允许请求
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * rl.rate
	
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.capacity {
			rl.tokens = rl.capacity
		}
		rl.lastRefill = now
	}
	
	// 检查是否有足够的令牌
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	
	return false
}

// GetTokens 获取当前令牌数
func (rl *RateLimiter) GetTokens() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.tokens
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	maxRetries    int           // 最大重试次数
	baseDelay     time.Duration // 基础延迟
	maxDelay      time.Duration // 最大延迟
	backoffFactor float64       // 退避因子
}

// NewRetryPolicy 创建新的重试策略
func NewRetryPolicy(maxRetries int, baseDelay, maxDelay time.Duration, backoffFactor float64) *RetryPolicy {
	return &RetryPolicy{
		maxRetries:    maxRetries,
		baseDelay:     baseDelay,
		maxDelay:      maxDelay,
		backoffFactor: backoffFactor,
	}
}

// ExecuteWithRetry 带重试的执行
func (rp *RetryPolicy) ExecuteWithRetry(fn func() error) error {
	var lastErr error
	
	for i := 0; i < rp.maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		if i < rp.maxRetries-1 {
			// 计算延迟时间
			delay := time.Duration(float64(rp.baseDelay) * pow(rp.backoffFactor, float64(i)))
			if delay > rp.maxDelay {
				delay = rp.maxDelay
			}
			
			time.Sleep(delay)
		}
	}
	
	return lastErr
}

// pow 计算幂
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// ErrorCollector 错误收集器
type ErrorCollector struct {
	errors      []error
	maxErrors   int
	mu          sync.Mutex
}

// NewErrorCollector 创建新的错误收集器
func NewErrorCollector(maxErrors int) *ErrorCollector {
	return &ErrorCollector{
		errors:    make([]error, 0, maxErrors),
		maxErrors: maxErrors,
	}
}

// Collect 收集错误
func (ec *ErrorCollector) Collect(err error) {
	if err == nil {
		return
	}
	
	ec.mu.Lock()
	defer ec.mu.Unlock()
	
	ec.errors = append(ec.errors, err)
	
	// 保持错误数量不超过最大值
	if len(ec.errors) > ec.maxErrors {
		ec.errors = ec.errors[1:]
	}
}

// GetErrors 获取错误
func (ec *ErrorCollector) GetErrors() []error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	
	// 返回错误的副本
	errors := make([]error, len(ec.errors))
	copy(errors, ec.errors)
	return errors
}

// Clear 清除错误
func (ec *ErrorCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.errors = ec.errors[:0]
}

// HasErrors 是否有错误
func (ec *ErrorCollector) HasErrors() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.errors) > 0
}

// GetErrorCount 获取错误数量
func (ec *ErrorCollector) GetErrorCount() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.errors)
}

// WorkerPool 工作池
type WorkerPool struct {
	workerCount int
	taskQueue   chan func()
	workers     []*worker
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
}

type worker struct {
	id       int
	taskQueue chan func()
	stopChan chan struct{}
	wg       *sync.WaitGroup
}

// NewWorkerPool 创建新的工作池
func NewWorkerPool(workerCount int, queueSize int) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		taskQueue:   make(chan func(), queueSize),
		stopChan:    make(chan struct{}),
	}
}

// Start 启动工作池
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	
	if wp.workers != nil {
		return // 已经启动
	}
	
	wp.workers = make([]*worker, wp.workerCount)
	
	for i := 0; i < wp.workerCount; i++ {
		wp.workers[i] = &worker{
			id:       i,
			taskQueue: wp.taskQueue,
			stopChan: wp.stopChan,
			wg:       &wp.wg,
		}
		wp.workers[i].start()
	}
	
	log.Printf("工作池已启动，工作线程数: %d", wp.workerCount)
}

// Stop 停止工作池
func (wp *WorkerPool) Stop() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	
	if wp.workers == nil {
		return // 已经停止
	}
	
	close(wp.stopChan)
	wp.wg.Wait()
	wp.workers = nil
	
	log.Printf("工作池已停止")
}

// Submit 提交任务
func (wp *WorkerPool) Submit(task func()) error {
	select {
	case wp.taskQueue <- task:
		return nil
	default:
		return errors.New("任务队列已满")
	}
}

// SubmitWithTimeout 带超时的提交任务
func (wp *WorkerPool) SubmitWithTimeout(task func(), timeout time.Duration) error {
	select {
	case wp.taskQueue <- task:
		return nil
	case <-time.After(timeout):
		return errors.New("提交任务超时")
	}
}

// GetStats 获取统计信息
func (wp *WorkerPool) GetStats() map[string]interface{} {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	
	stats := map[string]interface{}{
		"worker_count": wp.workerCount,
		"queue_size":   len(wp.taskQueue),
		"running":      wp.workers != nil,
	}
	
	return stats
}

func (w *worker) start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		
		for {
			select {
			case task := <-w.taskQueue:
				task()
			case <-w.stopChan:
				return
			}
		}
	}()
}

// ContextManager 上下文管理器
type ContextManager struct {
	timeout     time.Duration
	cancelFuncs map[string]context.CancelFunc
	mu          sync.RWMutex
}

// NewContextManager 创建新的上下文管理器
func NewContextManager(timeout time.Duration) *ContextManager {
	return &ContextManager{
		timeout:     timeout,
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

// CreateContext 创建上下文
func (cm *ContextManager) CreateContext(key string) context.Context {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// 如果已存在，先取消
	if cancel, exists := cm.cancelFuncs[key]; exists {
		cancel()
		delete(cm.cancelFuncs, key)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), cm.timeout)
	cm.cancelFuncs[key] = cancel
	
	return ctx
}

// CancelContext 取消上下文
func (cm *ContextManager) CancelContext(key string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if cancel, exists := cm.cancelFuncs[key]; exists {
		cancel()
		delete(cm.cancelFuncs, key)
	}
}

// CancelAll 取消所有上下文
func (cm *ContextManager) CancelAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for _, cancel := range cm.cancelFuncs {
		cancel()
	}
	
	cm.cancelFuncs = make(map[string]context.CancelFunc)
}

// GetContextCount 获取上下文数量
func (cm *ContextManager) GetContextCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.cancelFuncs)
}