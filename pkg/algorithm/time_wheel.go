package algorithm

import (
	"container/list"
	"sync"
	"time"
)

// Task 任务接口
type Task interface {
	Execute()
}

// TimeWheel 时间轮实现
type TimeWheel struct {
	interval      time.Duration                // 时间轮间隔，即每个槽位代表的时间
	slots         []*list.List                 // 槽位，每个槽位是一个任务列表
	currentPos    int                          // 当前指针位置
	slotNum       int                          // 槽位数量
	ticker        *time.Ticker                 // 定时器
	addTaskChan   chan *taskElement            // 添加任务通道
	stopChan      chan struct{}                // 停止通道
	taskMap       map[interface{}]*taskElement // 任务映射，用于快速查找任务
	mu            sync.RWMutex                  // 读写锁
	nextTimeWheel *TimeWheel                   // 下一级时间轮
}

// taskElement 任务元素
type taskElement struct {
	task    Task          // 任务
	delay   time.Duration // 延迟时间
	key     interface{}   // 任务唯一标识
	circle  int           // 圈数
	removed bool          // 是否已删除
}

// NewTimeWheel 创建时间轮
// interval: 时间轮间隔
// slotNum: 槽位数量
func NewTimeWheel(interval time.Duration, slotNum int) *TimeWheel {
	if interval <= 0 || slotNum <= 0 {
		return nil
	}

	tw := &TimeWheel{
		interval:    interval,
		slots:       make([]*list.List, slotNum),
		currentPos:  0,
		slotNum:     slotNum,
		addTaskChan: make(chan *taskElement, 1000),
		stopChan:    make(chan struct{}),
		taskMap:     make(map[interface{}]*taskElement),
	}

	// 初始化每个槽位
	for i := 0; i < slotNum; i++ {
		tw.slots[i] = list.New()
	}

	return tw
}

// Start 启动时间轮
func (tw *TimeWheel) Start() {
	tw.ticker = time.NewTicker(tw.interval)
	go tw.run()
}

// Stop 停止时间轮
func (tw *TimeWheel) Stop() {
	tw.stopChan <- struct{}{}
}

// AddTask 添加任务
func (tw *TimeWheel) AddTask(delay time.Duration, key interface{}, task Task) {
	if delay < 0 {
		return
	}

	tw.mu.Lock()
	// 如果任务已存在，先移除
	if element, ok := tw.taskMap[key]; ok {
		element.removed = true
		delete(tw.taskMap, key)
	}
	tw.mu.Unlock()

	tw.addTaskChan <- &taskElement{
		delay:  delay,
		key:    key,
		task:   task,
		circle: 0,
	}
}

// RemoveTask 移除任务
func (tw *TimeWheel) RemoveTask(key interface{}) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if element, ok := tw.taskMap[key]; ok {
		element.removed = true
		delete(tw.taskMap, key)
	}
	
	// 如果有下一级时间轮，也尝试移除
	if tw.nextTimeWheel != nil {
		tw.nextTimeWheel.RemoveTask(key)
	}
}

// HasTask 检查任务是否存在
func (tw *TimeWheel) HasTask(key interface{}) bool {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	
	_, exists := tw.taskMap[key]
	if !exists && tw.nextTimeWheel != nil {
		return tw.nextTimeWheel.HasTask(key)
	}
	return exists
}

// GetTaskCount 获取任务数量
func (tw *TimeWheel) GetTaskCount() int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	
	count := len(tw.taskMap)
	if tw.nextTimeWheel != nil {
		count += tw.nextTimeWheel.GetTaskCount()
	}
	return count
}

// run 运行时间轮
func (tw *TimeWheel) run() {
	for {
		select {
		case <-tw.ticker.C:
			tw.tickHandler()
		case task := <-tw.addTaskChan:
			tw.addTask(task)
		case <-tw.stopChan:
			tw.ticker.Stop()
			return
		}
	}
}

// tickHandler 处理时钟滴答
func (tw *TimeWheel) tickHandler() {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	// 获取当前槽位的任务列表
	currentList := tw.slots[tw.currentPos]

	// 遍历任务列表
	for e := currentList.Front(); e != nil; {
		task := e.Value.(*taskElement)

		// 获取下一个元素，因为当前元素可能会被删除
		next := e.Next()

		// 如果任务已被标记为删除，直接移除
		if task.removed {
			currentList.Remove(e)
			e = next
			continue
		}

		// 如果任务还需要等待下一圈
		if task.circle > 0 {
			task.circle--
			e = next
			continue
		}

		// 执行任务
		go task.task.Execute()

		// 从任务列表和映射中移除
		currentList.Remove(e)
		delete(tw.taskMap, task.key)

		e = next
	}

	// 移动时间轮指针
	tw.currentPos = (tw.currentPos + 1) % tw.slotNum
}

// addTask 添加任务到时间轮
func (tw *TimeWheel) addTask(task *taskElement) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	// 计算需要的延迟槽位数
	delaySlots := int(task.delay / tw.interval)

	// 如果延迟时间超过了时间轮的周期，需要创建下一级时间轮
	if delaySlots >= tw.slotNum {
		if tw.nextTimeWheel == nil {
			tw.nextTimeWheel = NewTimeWheel(tw.interval*time.Duration(tw.slotNum), tw.slotNum)
			tw.nextTimeWheel.Start()
		}

		// 将任务添加到下一级时间轮
		tw.nextTimeWheel.AddTask(task.delay, task.key, task.task)
		return
	}

	// 计算任务应该放入的槽位
	pos := (tw.currentPos + delaySlots) % tw.slotNum

	// 计算任务需要等待的圈数
	circle := delaySlots / tw.slotNum

	// 更新任务信息
	task.circle = circle

	// 将任务添加到对应槽位
	tw.slots[pos].PushBack(task)

	// 添加到任务映射
	tw.taskMap[task.key] = task
}
