package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// Version 表示一个语义化版本，包括主版本号、次版本号和修订号。
type Version struct {
	Major int // 主版本号
	Minor int // 次版本号
	Patch int // 修订号
}

// NewVersion 解析类似 "v0.0.1" 的版本字符串，并返回 Version 结构体。
// 如果格式有误，则返回错误。
func NewVersion(verStr string) (*Version, error) {
	verStr = strings.TrimPrefix(verStr, "v") // 去除前缀 'v'
	parts := strings.Split(verStr, ".")      // 按点分割
	if len(parts) != 3 {
		return nil, fmt.Errorf("版本格式不正确: %s", verStr)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, err
	}
	return &Version{Major: major, Minor: minor, Patch: patch}, nil
}

// String 格式化输出版本号，格式为 "v{major}.{minor}.{patch}"
func (v *Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Increment 对版本进行累加，修订号满 100 则次版本号加 1，次版本号满 100 则主版本号加 1。
func (v *Version) Increment() {
	v.Patch++
	if v.Patch == 100 {
		v.Patch = 0
		v.Minor++
		if v.Minor == 100 {
			v.Minor = 0
			v.Major++
		}
	}
}

// Compare 比较两个版本号。
// 返回 1 表示当前版本大于 other，-1 表示小于，0 表示相等。
func (v *Version) Compare(other *Version) int {
	if v.Major > other.Major {
		return 1
	} else if v.Major < other.Major {
		return -1
	}
	if v.Minor > other.Minor {
		return 1
	} else if v.Minor < other.Minor {
		return -1
	}
	if v.Patch > other.Patch {
		return 1
	} else if v.Patch < other.Patch {
		return -1
	}
	return 0
}
