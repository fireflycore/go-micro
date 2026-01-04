package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
)

// StructConvert 结构体转换
// 将 source 结构体转换为 target 结构体，通过 JSON 序列化/反序列化实现
// 注意：这会忽略性能，仅用于非关键路径或简单转换
func StructConvert[S any, T any](source *S, target *T) {
	marshal, mErr := json.Marshal(source)
	if mErr != nil {
		fmt.Println(mErr.Error())
		return
	}
	if err := json.Unmarshal(marshal, &target); err != nil {
		fmt.Println(err.Error())
		return
	}
}

// DiffStruct 比较两个结构体，返回发生改变的字段
// old: 旧结构体
// new: 新结构体
// ignore: 忽略比较的字段名列表
// 返回: 变更字段名 -> 新值的映射
func DiffStruct[O any, N any](old O, new N, ignore []string) map[string]interface{} {
	ro := reflect.ValueOf(old)
	rn := reflect.ValueOf(new)

	if ro.Kind() != reflect.Struct && (ro.Kind() != reflect.Ptr || ro.Elem().Kind() != reflect.Struct) {
		return nil
	}
	if rn.Kind() != reflect.Struct && (rn.Kind() != reflect.Ptr || rn.Elem().Kind() != reflect.Struct) {
		return nil
	}

	if ro.Kind() == reflect.Ptr {
		ro = ro.Elem()
	}
	if rn.Kind() == reflect.Ptr {
		rn = rn.Elem()
	}

	variant := make(map[string]interface{})

	for i := 0; i < rn.NumField(); i++ {
		k := rn.Type().Field(i).Name

		if slices.Contains(ignore, k) || !ro.FieldByName(k).IsValid() {
			continue
		}

		nv := rn.Field(i).Interface()
		ov := ro.FieldByName(k).Interface()

		if !reflect.DeepEqual(ov, nv) {
			variant[k] = nv
		}
	}

	return variant
}
