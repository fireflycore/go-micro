package config

import (
	"context"
	"encoding/json"
	"fmt"
)

// LocalLoaderFunc 定义本地配置加载函数签名。
// 典型实现是按 fileName 读取配置文件并反序列化到 target。
type LocalLoaderFunc func(fileName string, target any) error

// RemoteLoaderFunc 定义远程配置读取函数签名。
// 典型实现是从配置中心按 appId + group + key 获取配置文本。
type RemoteLoaderFunc func(appId, group, key string) (string, error)

// PayloadDecodeFunc 定义配置内容解码函数签名。
// 用于处理整份配置为密文的场景：先解密完整内容，再反序列化到 target。
type PayloadDecodeFunc func(content string, secret []byte, target any) error

// LoaderParams 描述加载后端配置对象所需参数。
// 该结构把 local / remote 两条加载链路的输入参数统一起来。
type LoaderParams struct {
	// LoadMode 指定加载模式：local / remote。
	LoadMode string
	// AppId 远程模式下用于定位配置所属应用。
	AppId string
	// AppSecret 远程模式下用于解密整份配置内容（如果是密文）。
	AppSecret []byte
	// Group 配置分组（如 database）。
	Group string
	// Key 配置名（如 consul / etcd / k8s）。
	Key string
	// FileName 本地模式下使用的文件名（如 consul.json）。
	FileName string
}

// StoreParams 描述从 Store 读取一条配置并解析所需参数。
// 当 Key 某些字段为空时，会用 AppId / Env / Group / Name 回填。
type StoreParams struct {
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
	// AppSecret 当读取到密文配置项时用于解码。
	AppSecret []byte
}

// LoadConfig 按 local / remote 规则加载并解析后端配置。
func LoadConfig[T any](params LoaderParams, localLoad LocalLoaderFunc, remoteLoad RemoteLoaderFunc, payloadDecode PayloadDecodeFunc) (T, error) {
	var target, zero T

	switch params.LoadMode {
	case "local":
		if localLoad == nil {
			return zero, ErrLocalLoaderIsNil
		}

		if err := localLoad(params.FileName, &target); err != nil {
			return zero, err
		}

		return target, nil
	case "remote":
		if remoteLoad == nil {
			return zero, ErrRemoteLoaderIsNil
		}

		row, err := remoteLoad(params.AppId, params.Group, params.Key)
		if err != nil {
			return zero, err
		}

		if payloadDecode != nil {
			if err = payloadDecode(row, params.AppSecret, &target); err != nil {
				return zero, err
			}
			return target, nil
		}

		if err = json.Unmarshal([]byte(row), &target); err != nil {
			return zero, err
		}

		return target, nil
	default:
		return zero, fmt.Errorf("%w: %s", ErrUnsupportedLoadMode, params.LoadMode)
	}
}

// LoadStoreConfig 从 Store 读取当前配置并解析为目标类型 T。
// 当 Raw.Encrypted=true 时，会先通过 payloadDecode 解密整份内容，再解析为目标结构。
func LoadStoreConfig[T any](ctx context.Context, store Store, params StoreParams, payloadDecode PayloadDecodeFunc) (T, error) {
	var target, zero T

	if store == nil {
		return zero, ErrStoreIsNil
	}

	key := params.Key
	if key.AppId == "" {
		key.AppId = params.AppId
	}
	if key.Env == "" {
		key.Env = params.Env
	}
	if key.Group == "" {
		key.Group = params.Group
	}
	if key.Name == "" {
		key.Name = params.Name
	}

	raw, err := store.Get(ctx, key)
	if err != nil {
		return zero, err
	}

	if raw.Encrypted {
		if payloadDecode == nil {
			return zero, ErrPayloadDecoderIsNil
		}

		if err = payloadDecode(string(raw.Content), params.AppSecret, &target); err != nil {
			return zero, err
		}

		return target, nil
	}

	if err = json.Unmarshal(raw.Content, &target); err != nil {
		return zero, err
	}

	return target, nil
}
