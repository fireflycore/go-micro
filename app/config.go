package app

import (
	"errors"

	"github.com/google/uuid"
)

type Config struct {
	// 应用id
	Id string `json:"id"`
	// 应用环境
	Env string `json:"env"`
	// 应用名称
	Name string `json:"name"`
	// 应用密钥
	Secret string `json:"secret"`
	// 应用版本
	Version string `json:"version"`
	// 实例id
	InstanceId string `json:"instance_id"`
}

func (c *Config) Bootstrap() error {
	if c.Id == "" {
		return errors.New("app.id is empty")
	}
	if c.Env == "" {
		return errors.New("app.env is empty")
	}
	if c.Name == "" {
		return errors.New("app.name is empty")
	}
	if c.Secret == "" {
		return errors.New("app.secret is empty")
	}
	if c.Version == "" {
		return errors.New("app.version is empty")
	}
	c.InstanceId = uuid.Must(uuid.NewV7()).String()
	return nil
}
