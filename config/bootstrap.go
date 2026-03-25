package config

import (
	"context"
	"encoding/json"
	"fmt"
)

// LocalConfigLoader 定义“本地配置加载器”函数签名。
// 典型实现是：按 fileName 读取 JSON 文件并反序列化到 target。
type LocalConfigLoader func(fileName string, target any) error

// RemoteConfigGetter 定义“远程配置获取器”函数签名。
// 典型实现是：从数据库/配置中心按 appId+group+key 取到配置字符串。
type RemoteConfigGetter func(appId, group, key string) (string, error)

// PayloadDecoder 定义“配置内容解码器”函数签名。
// 用于处理密文场景（例如 base64 + decrypt + decompress + json）。
type PayloadDecoder func(content string, secret []byte, target any) error

// StoreBootstrapRequest 描述“初始化后端配置对象”所需参数。
// 该结构用于把 local/remote 两条加载链路的入参统一起来。
type StoreBootstrapRequest struct {
	// LoadMode 指定加载模式：local / remote。
	LoadMode string
	// AppId 远程模式下用于定位配置所属应用。
	AppId string
	// AppSecret 远程模式下用于解密配置内容（如果是密文）。
	AppSecret []byte
	// Group 配置分组（如 database）。
	Group string
	// Key 配置名（如 consul / etcd / k8s）。
	Key string
	// FileName 本地模式下使用的文件名（如 consul.json）。
	FileName string
}

// StoreReadRequest 描述“从 Store 读取一条配置并解析”所需参数。
// 如果 Key 的某些字段为空，会用 AppId/Env/Group/Name 做回填。
type StoreReadRequest struct {
	// Key 是优先使用的业务键。
	Key Key
	// AppId 用于回填 Key.AppId。
	AppId string
	// Env 用于回填 Key.Env。
	Env string
	// Group 用于回填 Key.Group。
	Group string
	// Name 用于回填 Key.Name。
	Name string
	// AppSecret 当读取到密文配置时用于解码。
	AppSecret []byte
}

// DecodeBootstrapConfig 按 local/remote 规则加载并解析后端配置。
func DecodeBootstrapConfig[T any](request StoreBootstrapRequest, localLoader LocalConfigLoader, remoteGetter RemoteConfigGetter, payloadDecoder PayloadDecoder) (T, error) {
	var target T

	switch request.LoadMode {
	case "local":
		if localLoader == nil {
			var zero T
			return zero, fmt.Errorf("config local loader is nil")
		}
		if err := localLoader(request.FileName, &target); err != nil {
			var zero T
			return zero, err
		}
		return target, nil
	case "remote":
		if remoteGetter == nil {
			var zero T
			return zero, fmt.Errorf("config remote getter is nil")
		}
		row, err := remoteGetter(request.AppId, request.Group, request.Key)
		if err != nil {
			var zero T
			return zero, err
		}
		if payloadDecoder != nil {
			if err = payloadDecoder(row, request.AppSecret, &target); err != nil {
				var zero T
				return zero, err
			}
			return target, nil
		}
		if err = json.Unmarshal([]byte(row), &target); err != nil {
			var zero T
			return zero, err
		}
		return target, nil
	default:
		var zero T
		return zero, fmt.Errorf("unsupported load mode: %s", request.LoadMode)
	}
}

// DecodeStoreJSON 从 Store 读取配置并按 JSON 规则解析。
func DecodeStoreJSON[T any](ctx context.Context, store Store, request StoreReadRequest, payloadDecoder PayloadDecoder) (T, error) {
	var target T

	if store == nil {
		var zero T
		return zero, ErrStoreIsNil
	}

	key := request.Key
	if key.AppId == "" {
		key.AppId = request.AppId
	}
	if key.Env == "" {
		key.Env = request.Env
	}
	if key.Group == "" {
		key.Group = request.Group
	}
	if key.Name == "" {
		key.Name = request.Name
	}

	item, err := store.Get(ctx, key)
	if err != nil {
		var zero T
		return zero, err
	}

	if item.Encrypted {
		if payloadDecoder == nil {
			var zero T
			return zero, fmt.Errorf("config payload decoder is nil")
		}
		if err = payloadDecoder(string(item.Content), request.AppSecret, &target); err != nil {
			var zero T
			return zero, err
		}
		return target, nil
	}

	if err = json.Unmarshal(item.Content, &target); err != nil {
		var zero T
		return zero, err
	}
	return target, nil
}
