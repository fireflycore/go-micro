package config

import "time"

// Key 描述一条配置在存储中的业务主键。
type Key struct {
	// Group 表示配置分组。
	Group string `json:"group"`
	// Name 表示具体配置名。
	Name string `json:"name"`
	// Env 表示环境，如 dev/staging/prod。
	Env string `json:"env"`

	// AppId 表示应用标识。
	AppId string `json:"app_id"`
	// TenantId 表示租户标识，用于多租户隔离。
	TenantId string `json:"tenant_id"`
}

// Item 表示一条可发布、可读取的配置内容。
type Item struct {
	// Version 表示配置版本号。
	Version string `json:"version"`
	// Content 是配置原始内容。
	Content []byte `json:"content"`
	// Encrypted 标识 Content 是否已加密。
	Encrypted bool `json:"encrypted"`
	// Meta 记录扩展元信息。
	Meta map[string]string `json:"meta"`
	// UpdatedAt 表示最近更新时间。
	UpdatedAt time.Time `json:"updated_at"`
	// UpdatedBy 表示最近更新人。
	UpdatedBy string `json:"updated_by"`
}

// Meta 表示一组配置的版本游标信息。
type Meta struct {
	// CurrentVersion 表示当前生效版本。
	CurrentVersion string `json:"current_version"`
	// LatestVersion 表示最新发布版本。
	LatestVersion string `json:"latest_version"`
}

// Query 表示运行时按上下文查询配置的入参。
type Query struct {
	// Key 是基础配置键。
	Key Key `json:"key"`

	// UserId 表示请求上下文中的用户标识。
	UserId string `json:"user_id"`
	// AppId 表示请求上下文中的应用标识。
	AppId string `json:"app_id"`
	// TenantId 表示请求上下文中的租户标识。
	TenantId string `json:"tenant_id"`

	// Tags 表示额外标签条件。
	Tags map[string]string `json:"tags"`
}

// EventType 表示配置变更事件类型。
type EventType int

const (
	// EventPut 表示新增或更新事件。
	EventPut EventType = iota
	// EventDelete 表示删除事件。
	EventDelete
)

// WatchEvent 表示 watch 通知事件。
type WatchEvent struct {
	// Type 表示事件类型。
	Type EventType `json:"type"`
	// Key 表示事件对应的配置键。
	Key Key `json:"key"`
	// Item 表示事件携带的配置内容。
	Item *Item `json:"item"`
}
