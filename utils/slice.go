package utils

// DiffSlice 比较两个切片的差异
// old, new: 旧切片和新切片
// 返回:
//   - add: 新增的元素
//   - remove: 删除的元素
func DiffSlice[T comparable](old, new []T) (add, remove []T) {
	// 使用双map实现O(1)存在性检查 + 自动去重
	oldSet := make(map[T]struct{}, len(old))
	newSet := make(map[T]struct{}, len(new))

	// 初始化旧数据集（保持原始顺序用于删除检测）
	for _, item := range old {
		oldSet[item] = struct{}{}
	}

	// 初始化新数据集（保持原始顺序用于新增检测）
	for _, item := range new {
		newSet[item] = struct{}{}
	}

	// 检测新增项（使用新集合顺序保证确定性）
	add = make([]T, 0, len(newSet))
	for item := range newSet {
		if _, exists := oldSet[item]; !exists {
			add = append(add, item)
		}
	}

	// 检测删除项（使用旧集合顺序保证确定性）
	remove = make([]T, 0, len(oldSet))
	for item := range oldSet {
		if _, exists := newSet[item]; !exists {
			remove = append(remove, item)
		}
	}

	return add, remove
}

// FilterSlice 过滤切片
// array: 待过滤的切片
// fn: 过滤函数，返回 true 保留，返回 false 丢弃
func FilterSlice[T any](array []T, fn func(index int, item T) bool) []T {
	var temp []T
	for index, item := range array {
		if fn(index, item) {
			temp = append(temp, item)
		}
	}
	return temp
}
