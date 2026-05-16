package config

import "context"

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
	// Codec 用于配置内容的结构化编解码，未设置时默认使用 JSON。
	Codec Codec
	// Encryptor 用于密文配置的加解密。
	Encryptor Encryptor
	// Compressor 用于配置内容的压缩与解压。
	Compressor Compressor
}

// LoadStoreConfig 从 Store 读取当前配置并解析为目标类型 T。
// 配置内容统一走 payload 处理流程：Base64 解码 -> 按需解密 -> 解压 -> 反序列化。
func LoadStoreConfig[T any](ctx context.Context, store Store, params StoreParams) (T, error) {
	var target, zero T

	if store == nil {
		return zero, ErrStoreIsNil
	}

	// 回填 Key 字段
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

	// 从 Store 读取配置
	raw, err := store.Get(ctx, key)
	if err != nil {
		return zero, err
	}

	if err = UnmarshalPayload(string(raw.Content), raw.Encrypted, params.AppSecret, &target, params.Compressor, params.Encryptor, params.Codec); err != nil {
		return zero, err
	}

	return target, nil
}
