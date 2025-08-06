package algorithm

import (
	"strings"
	"sync"
)

// TrieNode 前缀树节点
type TrieNode struct {
	children map[rune]*TrieNode // 子节点
	isEnd    bool               // 是否是单词结尾
	value    interface{}        // 节点值
}

// Trie 前缀树
type Trie struct {
	root *TrieNode    // 根节点
	mu   sync.RWMutex // 读写锁
}

// NewTrie 创建新的前缀树
func NewTrie() *Trie {
	return &Trie{
		root: &TrieNode{
			children: make(map[rune]*TrieNode),
		},
	}
}

// Insert 插入字符串
func (t *Trie) Insert(key string, value interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	for _, ch := range key {
		if _, ok := node.children[ch]; !ok {
			node.children[ch] = &TrieNode{
				children: make(map[rune]*TrieNode),
			}
		}
		node = node.children[ch]
	}
	node.isEnd = true
	node.value = value
}

// Search 搜索字符串
func (t *Trie) Search(key string) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	for _, ch := range key {
		if _, ok := node.children[ch]; !ok {
			return nil, false
		}
		node = node.children[ch]
	}
	return node.value, node.isEnd
}

// StartsWith 检查是否有以指定前缀开头的字符串
func (t *Trie) StartsWith(prefix string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	for _, ch := range prefix {
		if _, ok := node.children[ch]; !ok {
			return false
		}
		node = node.children[ch]
	}
	return true
}

// Delete 删除字符串
func (t *Trie) Delete(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.delete(t.root, key, 0)
}

// delete 递归删除字符串
func (t *Trie) delete(node *TrieNode, key string, depth int) bool {
	// 如果已经到达字符串末尾
	if depth == len(key) {
		// 如果不是单词结尾，则不存在该单词
		if !node.isEnd {
			return false
		}

		// 标记为非单词结尾
		node.isEnd = false
		node.value = nil

		// 如果没有子节点，可以删除该节点
		return len(node.children) == 0
	}

	// 获取当前字符
	ch := rune(key[depth])

	// 如果子节点不存在，则不存在该单词
	child, ok := node.children[ch]
	if !ok {
		return false
	}

	// 递归删除子节点
	shouldDeleteChild := t.delete(child, key, depth+1)

	// 如果应该删除子节点
	if shouldDeleteChild {
		delete(node.children, ch)

		// 如果当前节点不是单词结尾且没有其他子节点，可以删除该节点
		return !node.isEnd && len(node.children) == 0
	}

	return false
}

// FindAllWithPrefix 查找所有具有指定前缀的字符串
func (t *Trie) FindAllWithPrefix(prefix string) map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]interface{})

	// 找到前缀对应的节点
	node := t.root
	for _, ch := range prefix {
		if _, ok := node.children[ch]; !ok {
			return result
		}
		node = node.children[ch]
	}

	// 从前缀节点开始，收集所有单词
	t.collectWords(node, prefix, result)

	return result
}

// collectWords 收集所有单词
func (t *Trie) collectWords(node *TrieNode, prefix string, result map[string]interface{}) {
	// 如果是单词结尾，添加到结果中
	if node.isEnd {
		result[prefix] = node.value
	}

	// 遍历所有子节点
	for ch, child := range node.children {
		t.collectWords(child, prefix+string(ch), result)
	}
}

// PatriciaTrie 压缩前缀树（Patricia树）
type PatriciaTrie struct {
	root *PatriciaNode // 根节点
	mu   sync.RWMutex  // 读写锁
}

// PatriciaNode 压缩前缀树节点
type PatriciaNode struct {
	prefix   string                 // 前缀
	children map[rune]*PatriciaNode // 子节点
	isEnd    bool                   // 是否是单词结尾
	value    interface{}            // 节点值
}

// NewPatriciaTrie 创建新的压缩前缀树
func NewPatriciaTrie() *PatriciaTrie {
	return &PatriciaTrie{
		root: &PatriciaNode{
			prefix:   "",
			children: make(map[rune]*PatriciaNode),
		},
	}
}

// Insert 插入字符串
func (pt *PatriciaTrie) Insert(key string, value interface{}) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.root = pt.insert(pt.root, key, value)
}

// insert 递归插入字符串
func (pt *PatriciaTrie) insert(node *PatriciaNode, key string, value interface{}) *PatriciaNode {
	// 如果节点为空，创建新节点
	if node == nil {
		return &PatriciaNode{
			prefix:   key,
			children: make(map[rune]*PatriciaNode),
			isEnd:    true,
			value:    value,
		}
	}

	// 如果键为空，标记当前节点为单词结尾
	if key == "" {
		node.isEnd = true
		node.value = value
		return node
	}

	// 找到当前节点前缀和键的最长公共前缀
	i := 0
	for i < len(node.prefix) && i < len(key) && node.prefix[i] == key[i] {
		i++
	}

	// 如果没有公共前缀，需要分裂
	if i == 0 {
		// 创建新节点
		newNode := &PatriciaNode{
			prefix:   key,
			children: make(map[rune]*PatriciaNode),
			isEnd:    true,
			value:    value,
		}

		// 如果键的第一个字符小于节点前缀的第一个字符，插入到前面
		if len(key) > 0 && len(node.prefix) > 0 && key[0] < node.prefix[0] {
			newRoot := &PatriciaNode{
				prefix:   "",
				children: make(map[rune]*PatriciaNode),
			}
			newRoot.children[rune(key[0])] = newNode
			newRoot.children[rune(node.prefix[0])] = node
			return newRoot
		}

		// 否则，插入到当前节点的子节点
		if len(key) > 0 {
			node.children[rune(key[0])] = newNode
		}
		return node
	}

	// 如果公共前缀等于节点前缀，继续在子节点中插入
	if i == len(node.prefix) {
		remainingKey := key[i:]
		if len(remainingKey) == 0 {
			node.isEnd = true
			node.value = value
			return node
		}

		firstChar := rune(remainingKey[0])
		child, ok := node.children[firstChar]
		if !ok {
			node.children[firstChar] = &PatriciaNode{
				prefix:   remainingKey,
				children: make(map[rune]*PatriciaNode),
				isEnd:    true,
				value:    value,
			}
		} else {
			node.children[firstChar] = pt.insert(child, remainingKey, value)
		}
		return node
	}

	// 如果公共前缀小于节点前缀，需要分裂节点
	commonPrefix := node.prefix[:i]
	remainingNodePrefix := node.prefix[i:]
	remainingKey := key[i:]

	// 创建分裂后的节点
	splitNode := &PatriciaNode{
		prefix:   commonPrefix,
		children: make(map[rune]*PatriciaNode),
	}

	// 原节点成为分裂节点的子节点
	origNode := &PatriciaNode{
		prefix:   remainingNodePrefix,
		children: node.children,
		isEnd:    node.isEnd,
		value:    node.value,
	}

	splitNode.children[rune(remainingNodePrefix[0])] = origNode

	// 如果剩余键不为空，创建新子节点
	if len(remainingKey) > 0 {
		splitNode.children[rune(remainingKey[0])] = &PatriciaNode{
			prefix:   remainingKey,
			children: make(map[rune]*PatriciaNode),
			isEnd:    true,
			value:    value,
		}
	} else {
		splitNode.isEnd = true
		splitNode.value = value
	}

	return splitNode
}

// Search 搜索字符串
func (pt *PatriciaTrie) Search(key string) (interface{}, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	node := pt.root
	for len(key) > 0 {
		// 如果键以节点前缀开头
		if strings.HasPrefix(key, node.prefix) {
			key = key[len(node.prefix):]

			// 如果键为空，表示找到了
			if key == "" {
				return node.value, node.isEnd
			}

			// 否则继续在子节点中搜索
			firstChar := rune(key[0])
			child, ok := node.children[firstChar]
			if !ok {
				return nil, false
			}
			node = child
		} else {
			return nil, false
		}
	}

	return node.value, node.isEnd
}

// StartsWith 检查是否有以指定前缀开头的字符串
func (pt *PatriciaTrie) StartsWith(prefix string) bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	node := pt.root
	for len(prefix) > 0 {
		// 如果前缀以节点前缀开头
		if strings.HasPrefix(prefix, node.prefix) {
			prefix = prefix[len(node.prefix):]

			// 如果前缀为空，表示找到了
			if prefix == "" {
				return true
			}

			// 否则继续在子节点中搜索
			firstChar := rune(prefix[0])
			child, ok := node.children[firstChar]
			if !ok {
				return false
			}
			node = child
		} else if strings.HasPrefix(node.prefix, prefix) {
			// 如果节点前缀以前缀开头，也是找到了
			return true
		} else {
			return false
		}
	}

	return true
}

// GetStats 获取统计信息
func (t *Trie) GetStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["node_count"] = t.countNodes(t.root)
	stats["word_count"] = t.countWords(t.root)
	stats["max_depth"] = t.maxDepth(t.root, 0)
	stats["memory_usage"] = t.estimateMemoryUsage()
	
	return stats
}

// countNodes 统计节点数量
func (t *Trie) countNodes(node *TrieNode) int {
	if node == nil {
		return 0
	}
	
	count := 1
	for _, child := range node.children {
		count += t.countNodes(child)
	}
	return count
}

// countWords 统计单词数量
func (t *Trie) countWords(node *TrieNode) int {
	if node == nil {
		return 0
	}
	
	count := 0
	if node.isEnd {
		count = 1
	}
	
	for _, child := range node.children {
		count += t.countWords(child)
	}
	return count
}

// maxDepth 计算最大深度
func (t *Trie) maxDepth(node *TrieNode, depth int) int {
	if node == nil {
		return depth
	}
	
	maxChildDepth := depth
	for _, child := range node.children {
		childDepth := t.maxDepth(child, depth+1)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}
	return maxChildDepth
}

// estimateMemoryUsage 估算内存使用量
func (t *Trie) estimateMemoryUsage() int {
	// 简化估算：每个节点约100字节
	return t.countNodes(t.root) * 100
}

// Size 获取大小（节点数量）
func (t *Trie) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.countNodes(t.root)
}

// Export 导出前缀树数据
func (t *Trie) Export() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	data := make(map[string]interface{})
	data["words"] = t.exportWords(t.root, "")
	data["stats"] = t.GetStats()
	
	return data
}

// exportWords 导出所有单词
func (t *Trie) exportWords(node *TrieNode, prefix string) map[string]interface{} {
	words := make(map[string]interface{})
	
	if node.isEnd {
		words[prefix] = node.value
	}
	
	for ch, child := range node.children {
		childWords := t.exportWords(child, prefix+string(ch))
		for k, v := range childWords {
			words[k] = v
		}
	}
	
	return words
}

// Import 导入前缀树数据
func (t *Trie) Import(data map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// 清空现有数据
	t.root = &TrieNode{
		children: make(map[rune]*TrieNode),
	}
	
	// 导入单词
	if words, ok := data["words"].(map[string]interface{}); ok {
		for key, value := range words {
			t.Insert(key, value)
		}
	}
	
	return nil
}
