package micro

import (
	"fmt"

	"github.com/fireflycore/go-micro/constant"
	"google.golang.org/grpc/metadata"
)

// UserContextMeta 用户上下文元信息
type UserContextMeta struct {
	Session  string `json:"session"`
	ClientIp string `json:"client_ip"`

	UserId   string `json:"user_id"`
	AppId    string `json:"app_id"`
	TenantId string `json:"tenant_id"`

	RoleIds []string `json:"role_ids"`
	OrgIds  []string `json:"org_ids"`
}

type ClientContextMeta struct {
	ClientIp    string `json:"client_ip"`
	AppVersion  string `json:"app_version"`
	AppLanguage string `json:"app_language"`
}

// ParseMetaKey 解析元信息key
func ParseMetaKey(md metadata.MD, key string) (string, error) {
	val := md.Get(key)

	if len(val) == 0 {
		return "", fmt.Errorf("%s parse error", key)
	}

	return val[0], nil
}

// ParseUserContextMeta 解析用户上下文元信息
func ParseUserContextMeta(md metadata.MD) (raw *UserContextMeta, err error) {
	raw = &UserContextMeta{}

	raw.Session, err = ParseMetaKey(md, constant.Session)
	if err != nil {
		return nil, err
	}
	raw.ClientIp, err = ParseMetaKey(md, constant.ClientIp)
	if err != nil {
		return nil, err
	}

	raw.UserId, err = ParseMetaKey(md, constant.UserId)
	if err != nil {
		return nil, err
	}
	raw.AppId, err = ParseMetaKey(md, constant.AppId)
	if err != nil {
		return nil, err
	}
	raw.TenantId, err = ParseMetaKey(md, constant.TenantId)
	if err != nil {
		return nil, err
	}

	raw.RoleIds = md.Get(constant.RoleIds)
	raw.OrgIds = md.Get(constant.OrgIds)

	return raw, nil
}

// ParseClientContextMeta 解析客户端上下文元信息
func ParseClientContextMeta(md metadata.MD) (raw *ClientContextMeta, err error) {
	raw = &ClientContextMeta{}

	raw.ClientIp, err = ParseMetaKey(md, constant.ClientIp)
	if err != nil {
		return nil, err
	}
	raw.AppVersion, err = ParseMetaKey(md, constant.AppVersion)
	if err != nil {
		return nil, err
	}
	raw.AppLanguage, err = ParseMetaKey(md, constant.AppLanguage)
	if err != nil {
		return nil, err
	}

	return raw, nil
}
