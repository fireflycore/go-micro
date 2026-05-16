package config

import (
	"context"
	"errors"
	"testing"
)

type testCompressor struct{}

func (testCompressor) Compress(data []byte) ([]byte, error) {
	return append([]byte("zip:"), data...), nil
}

func (testCompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) < 4 || string(data[:4]) != "zip:" {
		return nil, errors.New("invalid compressed payload")
	}
	return data[4:], nil
}

type testEncryptor struct{}

func (testEncryptor) Encrypt(data []byte, key []byte) ([]byte, error) {
	dst := make([]byte, 0, len("enc:")+len(key)+1+len(data))
	dst = append(dst, []byte("enc:")...)
	dst = append(dst, key...)
	dst = append(dst, ':')
	dst = append(dst, data...)
	return dst, nil
}

func (testEncryptor) Decrypt(data []byte, key []byte) ([]byte, error) {
	prefix := append(append([]byte("enc:"), key...), ':')
	if len(data) < len(prefix) || string(data[:len(prefix)]) != string(prefix) {
		return nil, errors.New("invalid encrypted payload")
	}
	return data[len(prefix):], nil
}

type testStore struct {
	raw *Raw
	err error
}

func (s *testStore) Get(context.Context, Key) (*Raw, error) {
	return s.raw, s.err
}

func (s *testStore) Put(context.Context, Key, *Raw) error {
	return nil
}

func (s *testStore) Delete(context.Context, Key) error {
	return nil
}

func TestEncodeDecodePayloadPlain(t *testing.T) {
	compressor := testCompressor{}
	source := []byte(`{"name":"demo"}`)

	encoded, err := EncodePayload(source, false, nil, compressor, nil)
	if err != nil {
		t.Fatalf("EncodePayload() error = %v", err)
	}

	decoded, err := DecodePayload(encoded, false, nil, compressor, nil)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}

	if string(decoded) != string(source) {
		t.Fatalf("DecodePayload() = %s, want %s", string(decoded), string(source))
	}
}

func TestEncodeDecodePayloadEncrypted(t *testing.T) {
	compressor := testCompressor{}
	encryptor := testEncryptor{}
	secret := []byte("app-secret")
	source := []byte(`{"name":"secure"}`)

	encoded, err := EncodePayload(source, true, secret, compressor, encryptor)
	if err != nil {
		t.Fatalf("EncodePayload() error = %v", err)
	}

	decoded, err := DecodePayload(encoded, true, secret, compressor, encryptor)
	if err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}

	if string(decoded) != string(source) {
		t.Fatalf("DecodePayload() = %s, want %s", string(decoded), string(source))
	}
}

func TestMarshalUnmarshalPayload(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}

	compressor := testCompressor{}
	encryptor := testEncryptor{}
	secret := []byte("marshal-secret")
	source := payload{Name: "config", Port: 8080}

	encoded, err := MarshalPayload(source, true, secret, compressor, encryptor, nil)
	if err != nil {
		t.Fatalf("MarshalPayload() error = %v", err)
	}

	var target payload
	if err = UnmarshalPayload(encoded, true, secret, &target, compressor, encryptor, nil); err != nil {
		t.Fatalf("UnmarshalPayload() error = %v", err)
	}

	if target != source {
		t.Fatalf("UnmarshalPayload() = %+v, want %+v", target, source)
	}
}

func TestLoadStoreConfigWithUnifiedPayloadPipeline(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}

	compressor := testCompressor{}
	encryptor := testEncryptor{}
	secret := []byte("store-secret")
	source := payload{Name: "redis", Port: 6379}

	encoded, err := MarshalPayload(source, true, secret, compressor, encryptor, nil)
	if err != nil {
		t.Fatalf("MarshalPayload() error = %v", err)
	}

	store := &testStore{
		raw: &Raw{
			Content:   []byte(encoded),
			Encrypted: true,
		},
	}

	target, err := LoadStoreConfig[payload](context.Background(), store, StoreParams{
		AppId:      "app",
		Env:        "prod",
		Group:      "database",
		Name:       "redis",
		AppSecret:  secret,
		Compressor: compressor,
		Encryptor:  encryptor,
})
	if err != nil {
		t.Fatalf("LoadStoreConfig() error = %v", err)
	}

	if target != source {
		t.Fatalf("LoadStoreConfig() = %+v, want %+v", target, source)
	}
}
