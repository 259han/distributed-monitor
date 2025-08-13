package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// ErrorType 错误类型
type ErrorType string

const (
	ErrorTypeSystem    ErrorType = "system"
	ErrorTypeNetwork   ErrorType = "network"
	ErrorTypeDatabase  ErrorType = "database"
	ErrorTypeBusiness  ErrorType = "business"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeTimeout   ErrorType = "timeout"
	ErrorTypeRetry     ErrorType = "retry"
)

// ErrorSeverity 错误严重程度
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ErrorDetail 错误详情
type ErrorDetail struct {
	Type      ErrorType   `json:"type"`
	Severity  ErrorSeverity `json:"severity"`
	Message   string      `json:"message"`
	Code      string      `json:"code,omitempty"`
	Context   interface{} `json:"context,omitempty"`
	Stack     string      `json:"stack,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
	Count     int         `json:"count"`
}

// ErrorHandler 错误处理器
type ErrorHandler struct {
	errorDetails map[string]*ErrorDetail
	maxErrors    int
	logFile      *os.File
	mu           sync.RWMutex
}

// NewErrorHandler 创建新的错误处理器
func NewErrorHandler(maxErrors int, logFilePath string) (*ErrorHandler, error) {
	eh := &ErrorHandler{
		errorDetails: make(map[string]*ErrorDetail),
		maxErrors:    maxErrors,
	}
	
	// 如果指定了日志文件路径，打开文件
	if logFilePath != "" {
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开错误日志文件失败: %v", err)
		}
		eh.logFile = file
	}
	
	return eh, nil
}

// HandleError 处理错误
func (eh *ErrorHandler) HandleError(err error, errorType ErrorType, severity ErrorSeverity, context interface{}) {
	if err == nil {
		return
	}
	
	eh.mu.Lock()
	defer eh.mu.Unlock()
	
	// 生成错误码
	code := eh.generateErrorCode(err)
	
	// 创建错误详情
	detail := &ErrorDetail{
		Type:      errorType,
		Severity:  severity,
		Message:   err.Error(),
		Code:      code,
		Context:   context,
		Stack:     string(debug.Stack()),
		Timestamp: time.Now(),
		Count:     1,
	}
	
	// 检查是否已存在相同错误
	if existing, exists := eh.errorDetails[code]; exists {
		existing.Count++
		existing.Timestamp = time.Now()
		detail = existing
	} else {
		eh.errorDetails[code] = detail
		
		// 如果错误数量超过限制，删除最旧的错误
		if len(eh.errorDetails) > eh.maxErrors {
			oldestCode := ""
			oldestTime := time.Now()
			
			for code, detail := range eh.errorDetails {
				if detail.Timestamp.Before(oldestTime) {
					oldestTime = detail.Timestamp
					oldestCode = code
				}
			}
			
			if oldestCode != "" {
				delete(eh.errorDetails, oldestCode)
			}
		}
	}
	
	// 记录错误日志
	eh.logError(detail)
	
	// 根据严重程度采取不同措施
	switch severity {
	case SeverityCritical:
		log.Printf("严重错误: %s [%s] %s", code, errorType, err.Error())
		// 可以在这里添加告警逻辑
	case SeverityHigh:
		log.Printf("高优先级错误: %s [%s] %s", code, errorType, err.Error())
	case SeverityMedium:
		log.Printf("中等优先级错误: %s [%s] %s", code, errorType, err.Error())
	case SeverityLow:
		log.Printf("低优先级错误: %s [%s] %s", code, errorType, err.Error())
	}
}

// logError 记录错误到文件
func (eh *ErrorHandler) logError(detail *ErrorDetail) {
	if eh.logFile == nil {
		return
	}
	
	// 序列化错误详情
	data, err := json.Marshal(detail)
	if err != nil {
		log.Printf("序列化错误详情失败: %v", err)
		return
	}
	
	// 写入文件
	_, err = eh.logFile.WriteString(string(data) + "\n")
	if err != nil {
		log.Printf("写入错误日志文件失败: %v", err)
	}
}

// generateErrorCode 生成错误码
func (eh *ErrorHandler) generateErrorCode(err error) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("ERR-%s-%d", timestamp, len(eh.errorDetails))
}

// GetErrors 获取错误列表
func (eh *ErrorHandler) GetErrors() []*ErrorDetail {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	
	errors := make([]*ErrorDetail, 0, len(eh.errorDetails))
	for _, detail := range eh.errorDetails {
		errors = append(errors, detail)
	}
	
	return errors
}

// GetErrorsByType 根据类型获取错误
func (eh *ErrorHandler) GetErrorsByType(errorType ErrorType) []*ErrorDetail {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	
	var errors []*ErrorDetail
	for _, detail := range eh.errorDetails {
		if detail.Type == errorType {
			errors = append(errors, detail)
		}
	}
	
	return errors
}

// GetErrorsBySeverity 根据严重程度获取错误
func (eh *ErrorHandler) GetErrorsBySeverity(severity ErrorSeverity) []*ErrorDetail {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	
	var errors []*ErrorDetail
	for _, detail := range eh.errorDetails {
		if detail.Severity == severity {
			errors = append(errors, detail)
		}
	}
	
	return errors
}

// ClearErrors 清除错误
func (eh *ErrorHandler) ClearErrors() {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	
	eh.errorDetails = make(map[string]*ErrorDetail)
}

// GetErrorCount 获取错误数量
func (eh *ErrorHandler) GetErrorCount() int {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return len(eh.errorDetails)
}

// Close 关闭错误处理器
func (eh *ErrorHandler) Close() error {
	if eh.logFile != nil {
		return eh.logFile.Close()
	}
	return nil
}

// ErrorReporter 错误报告器
type ErrorReporter struct {
	handler      *ErrorHandler
	reportPeriod time.Duration
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewErrorReporter 创建新的错误报告器
func NewErrorReporter(handler *ErrorHandler, reportPeriod time.Duration) *ErrorReporter {
	return &ErrorReporter{
		handler:      handler,
		reportPeriod: reportPeriod,
		stopChan:     make(chan struct{}),
	}
}

// Start 启动错误报告器
func (er *ErrorReporter) Start() {
	er.wg.Add(1)
	go er.run()
}

// Stop 停止错误报告器
func (er *ErrorReporter) Stop() {
	close(er.stopChan)
	er.wg.Wait()
}

// run 运行错误报告器
func (er *ErrorReporter) run() {
	defer er.wg.Done()
	
	ticker := time.NewTicker(er.reportPeriod)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			er.generateReport()
		case <-er.stopChan:
			return
		}
	}
}

// generateReport 生成错误报告
func (er *ErrorReporter) generateReport() {
	errors := er.handler.GetErrors()
	if len(errors) == 0 {
		return
	}
	
	log.Printf("错误报告生成时间: %s", time.Now().Format("2006-01-02 15:04:05"))
	log.Printf("总错误数量: %d", len(errors))
	
	// 按类型统计
	typeCount := make(map[ErrorType]int)
	severityCount := make(map[ErrorSeverity]int)
	
	for _, detail := range errors {
		typeCount[detail.Type]++
		severityCount[detail.Severity]++
	}
	
	log.Printf("按类型统计:")
	for errorType, count := range typeCount {
		log.Printf("  %s: %d", errorType, count)
	}
	
	log.Printf("按严重程度统计:")
	for severity, count := range severityCount {
		log.Printf("  %s: %d", severity, count)
	}
	
	// 显示最频繁的错误
	if len(errors) > 0 {
		log.Printf("最频繁的错误:")
		for i := 0; i < 5 && i < len(errors); i++ {
			detail := errors[i]
			log.Printf("  %s: %s (出现次数: %d)", detail.Code, detail.Message, detail.Count)
		}
	}
}

// PanicHandler 恐慌处理器
type PanicHandler struct {
	recoverFunc func(interface{})
}

// NewPanicHandler 创建新的恐慌处理器
func NewPanicHandler(recoverFunc func(interface{})) *PanicHandler {
	return &PanicHandler{
		recoverFunc: recoverFunc,
	}
}

// Recover 恢复恐慌
func (ph *PanicHandler) Recover() {
	if r := recover(); r != nil {
		log.Printf("发生恐慌: %v", r)
		log.Printf("堆栈信息: %s", debug.Stack())
		
		if ph.recoverFunc != nil {
			ph.recoverFunc(r)
		}
	}
}

// WrapPanic 包装恐慌恢复
func WrapPanic(fn func()) func() {
	return func() {
		defer NewPanicHandler(nil).Recover()
		fn()
	}
}

// ErrorGroup 错误组
type ErrorGroup struct {
	errors []error
	mu     sync.Mutex
}

// NewErrorGroup 创建新的错误组
func NewErrorGroup() *ErrorGroup {
	return &ErrorGroup{
		errors: make([]error, 0),
	}
}

// Add 添加错误
func (eg *ErrorGroup) Add(err error) {
	if err == nil {
		return
	}
	
	eg.mu.Lock()
	defer eg.mu.Unlock()
	
	eg.errors = append(eg.errors, err)
}

// Error 返回错误
func (eg *ErrorGroup) Error() string {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	
	if len(eg.errors) == 0 {
		return ""
	}
	
	return fmt.Sprintf("多个错误: %v", eg.errors)
}

// HasErrors 是否有错误
func (eg *ErrorGroup) HasErrors() bool {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	return len(eg.errors) > 0
}

// GetErrors 获取错误列表
func (eg *ErrorGroup) GetErrors() []error {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	
	errors := make([]error, len(eg.errors))
	copy(errors, eg.errors)
	return errors
}

// Clear 清除错误
func (eg *ErrorGroup) Clear() {
	eg.mu.Lock()
	defer eg.mu.Unlock()
	eg.errors = eg.errors[:0]
}

// RetryableError 可重试错误
type RetryableError struct {
	Err      error
	Retry    bool
	Interval time.Duration
}

// Error 实现error接口
func (re *RetryableError) Error() string {
	return re.Err.Error()
}

// NewRetryableError 创建新的可重试错误
func NewRetryableError(err error, retry bool, interval time.Duration) *RetryableError {
	return &RetryableError{
		Err:      err,
		Retry:    retry,
		Interval: interval,
	}
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	if re, ok := err.(*RetryableError); ok {
		return re.Retry
	}
	return false
}

// GetRetryInterval 获取重试间隔
func GetRetryInterval(err error) time.Duration {
	if re, ok := err.(*RetryableError); ok {
		return re.Interval
	}
	return 0
}