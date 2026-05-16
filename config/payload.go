package config

import (
	"encoding/base64"
	"encoding/json"
)

// EncodePayload 按统一规则把原始配置内容编码为可持久化字符串。
// 处理顺序固定为：压缩 -> 按需加密 -> Base64 编码。
func EncodePayload(content []byte, encrypted bool, secret []byte, compressor Compressor, encryptor Encryptor) (string, error) {
	if compressor == nil {
		return "", ErrCompressorIsNil
	}

	data, err := compressor.Compress(content)
	if err != nil {
		return "", err
	}

	if encrypted {
		if encryptor == nil {
			return "", ErrEncryptorIsNil
		}
		data, err = encryptor.Encrypt(data, secret)
		if err != nil {
			return "", err
		}
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// DecodePayload 按统一规则把持久化字符串还原为原始配置内容。
// 处理顺序固定为：Base64 解码 -> 按需解密 -> 解压。
func DecodePayload(content string, encrypted bool, secret []byte, compressor Compressor, encryptor Encryptor) ([]byte, error) {
	if compressor == nil {
		return nil, ErrCompressorIsNil
	}

	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, err
	}

	if encrypted {
		if encryptor == nil {
			return nil, ErrEncryptorIsNil
		}
		data, err = encryptor.Decrypt(data, secret)
		if err != nil {
			return nil, err
		}
	}

	return compressor.Decompress(data)
}

// MarshalPayload 把结构化配置对象编码为统一的持久化字符串。
func MarshalPayload(value any, encrypted bool, secret []byte, compressor Compressor, encryptor Encryptor, codec Codec) (string, error) {
	data, err := marshalPayloadValue(value, codec)
	if err != nil {
		return "", err
	}

	return EncodePayload(data, encrypted, secret, compressor, encryptor)
}

// UnmarshalPayload 把统一的持久化字符串解码到目标结构中。
func UnmarshalPayload(content string, encrypted bool, secret []byte, target any, compressor Compressor, encryptor Encryptor, codec Codec) error {
	data, err := DecodePayload(content, encrypted, secret, compressor, encryptor)
	if err != nil {
		return err
	}

	return unmarshalPayloadValue(data, target, codec)
}

func marshalPayloadValue(value any, codec Codec) ([]byte, error) {
	if codec != nil {
		return codec.Marshal(value)
	}
	return json.Marshal(value)
}

func unmarshalPayloadValue(data []byte, target any, codec Codec) error {
	if codec != nil {
		return codec.Unmarshal(data, target)
	}
	return json.Unmarshal(data, target)
}
