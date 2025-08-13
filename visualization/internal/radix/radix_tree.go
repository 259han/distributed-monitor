package radix

import (
	"errors"
	"strings"
)

// Node 基数树节点
type Node struct {
	Key      string
	Value    interface{}
	Children map[string]*Node
	IsLeaf   bool
}

// RadixTree 基数树
type RadixTree struct {
	root *Node
	size int
}

// NewRadixTree 创建新的基数树
func NewRadixTree() *RadixTree {
	return &RadixTree{
		root: &Node{
			Key:      "",
			Children: make(map[string]*Node),
		},
		size: 0,
	}
}

// Insert 插入键值对
func (t *RadixTree) Insert(key string, value interface{}) error {
	if key == "" {
		return errors.New("键不能为空")
	}

	t.size++
	return t.insert(t.root, key, value)
}

// insert 递归插入键值对
func (t *RadixTree) insert(node *Node, key string, value interface{}) error {
	// 查找最长公共前缀
	for prefix, child := range node.Children {
		commonPrefix := longestCommonPrefix(prefix, key)

		// 如果有公共前缀
		if commonPrefix != "" {
			// 如果公共前缀等于当前节点的键，继续向下查找
			if commonPrefix == prefix {
				return t.insert(child, key[len(prefix):], value)
			}

			// 如果公共前缀等于要插入的键，更新当前节点
			if commonPrefix == key {
				child.Value = value
				child.IsLeaf = true
				return nil
			}

			// 分裂节点
			// 1. 创建新的中间节点
			newNode := &Node{
				Key:      commonPrefix,
				Children: make(map[string]*Node),
			}

			// 2. 将原节点作为新节点的子节点
			remainingPrefix := prefix[len(commonPrefix):]
			originalChild := child
			originalChild.Key = remainingPrefix
			newNode.Children[remainingPrefix] = originalChild

			// 3. 将新节点替换原节点在父节点的位置
			delete(node.Children, prefix)
			node.Children[commonPrefix] = newNode

			// 4. 如果要插入的键与公共前缀不同，创建新的叶子节点
			if key != commonPrefix {
				remainingKey := key[len(commonPrefix):]
				newNode.Children[remainingKey] = &Node{
					Key:      remainingKey,
					Value:    value,
					Children: make(map[string]*Node),
					IsLeaf:   true,
				}
			} else {
				// 如果要插入的键就是公共前缀，更新新节点
				newNode.Value = value
				newNode.IsLeaf = true
			}

			return nil
		}
	}

	// 没有找到公共前缀，直接添加新节点
	node.Children[key] = &Node{
		Key:      key,
		Value:    value,
		Children: make(map[string]*Node),
		IsLeaf:   true,
	}

	return nil
}

// Search 搜索键对应的值
func (t *RadixTree) Search(key string) (interface{}, bool) {
	if key == "" {
		return nil, false
	}

	return t.search(t.root, key)
}

// search 递归搜索键对应的值
func (t *RadixTree) search(node *Node, key string) (interface{}, bool) {
	// 如果键为空且当前节点是叶子节点，返回当前节点的值
	if key == "" && node.IsLeaf {
		return node.Value, true
	}

	// 查找匹配的前缀
	for prefix, child := range node.Children {
		if strings.HasPrefix(key, prefix) {
			return t.search(child, key[len(prefix):])
		}
	}

	return nil, false
}

// Delete 删除键
func (t *RadixTree) Delete(key string) bool {
	if key == "" {
		return false
	}

	found := t.delete(t.root, key)
	if found {
		t.size--
	}
	return found
}

// delete 递归删除键
func (t *RadixTree) delete(node *Node, key string) bool {
	// 如果键为空且当前节点是叶子节点，标记为非叶子节点
	if key == "" && node.IsLeaf {
		node.IsLeaf = false
		node.Value = nil
		return true
	}

	// 查找匹配的前缀
	for prefix, child := range node.Children {
		if strings.HasPrefix(key, prefix) {
			remainingKey := key[len(prefix):]
			found := t.delete(child, remainingKey)

			// 如果子节点已经没有子节点且不是叶子节点，删除该子节点
			if found && len(child.Children) == 0 && !child.IsLeaf {
				delete(node.Children, prefix)
			}

			return found
		}
	}

	return false
}

// Size 返回树的大小
func (t *RadixTree) Size() int {
	return t.size
}

// PrefixSearch 前缀搜索，返回所有以prefix开头的键值对
func (t *RadixTree) PrefixSearch(prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	if prefix == "" {
		return result
	}

	// 查找前缀对应的节点
	node := t.root
	currentPrefix := ""

	for {
		found := false
		for nodePrefix, child := range node.Children {
			if strings.HasPrefix(prefix, currentPrefix+nodePrefix) {
				node = child
				currentPrefix += nodePrefix
				found = true
				break
			}
		}

		if !found || currentPrefix == prefix {
			break
		}
	}

	// 收集所有以该节点为根的键值对
	if strings.HasPrefix(prefix, currentPrefix) {
		t.collectPrefixMatches(node, currentPrefix, prefix, result)
	}

	return result
}

// collectPrefixMatches 收集所有前缀匹配的键值对
func (t *RadixTree) collectPrefixMatches(node *Node, currentPath, prefix string, result map[string]interface{}) {
	// 如果当前节点是叶子节点且当前路径以前缀开头，添加到结果中
	if node.IsLeaf && strings.HasPrefix(currentPath, prefix) {
		result[currentPath] = node.Value
	}

	// 递归处理所有子节点
	for nodePrefix, child := range node.Children {
		t.collectPrefixMatches(child, currentPath+nodePrefix, prefix, result)
	}
}

// longestCommonPrefix 查找两个字符串的最长公共前缀
func longestCommonPrefix(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}

	return a[:minLen]
}
