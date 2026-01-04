package utils

import "strings"

// LTree 树结构工具类
type LTree struct{}

func NewLTree() *LTree {
	return &LTree{}
}

// GetLTreeDepth 计算LTree路径的深度
func (t *LTree) GetLTreeDepth(path string) int {
	if path == "" {
		return 0
	}
	return strings.Count(path, ".") + 1
}

// BuildLTreePath 构建LTree路径
func (t *LTree) BuildLTreePath(parentPath, currentValue string) string {
	if parentPath == "" {
		return currentValue
	}
	return parentPath + "." + currentValue
}

// ExtractLastSegment 提取路径的最后一段
func (t *LTree) ExtractLastSegment(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// IsDescendantPath 检查路径是否是后代路径
func (t *LTree) IsDescendantPath(descendantPath, ancestorPath string) bool {
	if ancestorPath == "" {
		return true
	}
	return strings.HasPrefix(descendantPath, ancestorPath+".")
}

// ReplacePathPrefix 替换路径前缀
func (t *LTree) ReplacePathPrefix(oldPath, newPath, fullPath string) string {
	if !strings.HasPrefix(fullPath, oldPath+".") {
		return fullPath
	}
	return newPath + "." + strings.TrimPrefix(fullPath, oldPath+".")
}

// ReplaceCurrentPath 替换当前路径
func (t *LTree) ReplaceCurrentPath(oldPath, currentPath string) string {
	if oldPath == "" {
		return currentPath
	}
	parts := strings.Split(oldPath, ".")
	parts = parts[:len(parts)-1]
	parts = append(parts, currentPath)
	return strings.Join(parts, ".")
}
