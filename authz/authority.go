package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultServiceAuthorityRefreshBefore 是 service token 过期前的默认主动刷新窗口。
	DefaultServiceAuthorityRefreshBefore = time.Minute
)

var (
	// ErrServiceAuthorityFetchMissing 表示启用 service authority 时没有配置取 token 函数。
	ErrServiceAuthorityFetchMissing = errors.New("authz service authority fetch function is missing")
	// ErrServiceAuthorityTokenMissing 表示取到的 service token 为空，不能写入出站请求。
	ErrServiceAuthorityTokenMissing = errors.New("authz service authority token is missing")
	// ErrServiceAuthorityTokenExpiresAtMissing 表示 service token 没有明确过期时间，无法做动态轮换。
	ErrServiceAuthorityTokenExpiresAtMissing = errors.New("authz service authority token expires_at is missing")
	// ErrServiceAuthorityTokenExpired 表示取到的 service token 已经过期。
	ErrServiceAuthorityTokenExpired = errors.New("authz service authority token is expired")
)

// ServiceAuthorityConfig 描述服务间调用 service token 的主动刷新策略。
type ServiceAuthorityConfig struct {
	// RefreshBefore 表示 token 过期前多久主动刷新，例如 1m；为空使用默认值。
	RefreshBefore string `json:"refresh_before" yaml:"refresh_before"`
}

// ServiceAuthorityToken 表示 auth 服务签发的服务身份凭证及其过期时间。
type ServiceAuthorityToken struct {
	// Token 是最终写入 X-Firefly-Service-Authority 的服务 token。
	Token string
	// ExpiresAt 是 token 过期时间；目标链路要求 service token 必须可轮换，零值无效。
	ExpiresAt time.Time
}

// ServiceAuthorityFetchFunc 表示业务服务如何从 auth 服务获取 service token。
type ServiceAuthorityFetchFunc func(ctx context.Context) (*ServiceAuthorityToken, error)

// ServiceAuthorityProvider 表示出站调用在热路径上获取当前服务 token 的能力。
type ServiceAuthorityProvider interface {
	// ServiceAuthority 返回当前这一跳调用方服务身份 token。
	ServiceAuthority(ctx context.Context) (string, error)
}

// CachedServiceAuthorityProviderOptions 定义缓存型 service authority provider 的依赖。
type CachedServiceAuthorityProviderOptions struct {
	// Fetch 负责真正向 auth 服务签发或刷新 service token。
	Fetch ServiceAuthorityFetchFunc
	// RefreshBefore 表示 token 过期前多久主动刷新。
	RefreshBefore time.Duration
}

// CachedServiceAuthorityProvider 在进程内缓存 service token，并在过期前主动刷新。
type CachedServiceAuthorityProvider struct {
	// mu 保护 token 与 expiresAt，避免并发刷新时出现数据竞争。
	mu sync.Mutex
	// fetch 保存真正获取 service token 的业务函数。
	fetch ServiceAuthorityFetchFunc
	// refreshBefore 保存提前刷新窗口，避免临界过期 token 被写入出站请求。
	refreshBefore time.Duration
	// token 保存最近一次成功获取的 service token。
	token string
	// expiresAt 保存 token 过期时间；零值表示永久有效。
	expiresAt time.Time
}

// NewServiceAuthorityProvider 根据配置和取 token 函数构造缓存型 provider。
func NewServiceAuthorityProvider(cfg *ServiceAuthorityConfig, fetch ServiceAuthorityFetchFunc) (ServiceAuthorityProvider, error) {
	// 目标链路下 service authority provider 只要被构造，就必须能获取当前服务 token。
	if fetch == nil {
		return nil, ErrServiceAuthorityFetchMissing
	}
	// nil 配置表示使用默认刷新窗口，不表示禁用 service authority。
	if cfg == nil {
		cfg = &ServiceAuthorityConfig{}
	}
	// 解析刷新窗口，支持 30s、1m 等标准 duration。
	refreshBefore, err := parseServiceAuthorityRefreshBefore(cfg.RefreshBefore)
	if err != nil {
		return nil, err
	}
	// 构造可在多协程出站调用中复用的缓存 provider。
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		Fetch:         fetch,
		RefreshBefore: refreshBefore,
	})
	if err != nil {
		return nil, err
	}
	// 返回标准接口，调用方无需依赖具体缓存实现。
	return provider, nil
}

// NewCachedServiceAuthorityProvider 创建缓存型 service authority provider。
func NewCachedServiceAuthorityProvider(options CachedServiceAuthorityProviderOptions) (*CachedServiceAuthorityProvider, error) {
	// 取 token 函数是唯一必需依赖。
	if options.Fetch == nil {
		return nil, ErrServiceAuthorityFetchMissing
	}
	// 未指定刷新窗口时使用默认值。
	refreshBefore := options.RefreshBefore
	if refreshBefore <= 0 {
		refreshBefore = DefaultServiceAuthorityRefreshBefore
	}
	// 返回内部状态为空的 provider，首次调用时懒加载 token。
	return &CachedServiceAuthorityProvider{
		fetch:         options.Fetch,
		refreshBefore: refreshBefore,
	}, nil
}

// ServiceAuthority 返回当前可用的 service token。
func (p *CachedServiceAuthorityProvider) ServiceAuthority(ctx context.Context) (string, error) {
	// provider 为空时说明调用方装配错误，返回明确错误。
	if p == nil {
		return "", ErrServiceAuthorityFetchMissing
	}
	// 加锁保护缓存检查和刷新，避免并发同时向 auth 服务刷新 token。
	p.mu.Lock()
	defer p.mu.Unlock()

	// 缓存 token 仍处于刷新窗口外时直接复用。
	if p.isCachedTokenUsableLocked() {
		return p.token, nil
	}

	// 缓存不存在或即将过期时调用业务 fetch 函数刷新。
	token, err := p.fetch(ctx)
	if err != nil {
		return "", err
	}
	// 统一校验 fetch 结果，避免空 token 或过期 token 污染缓存。
	if err := validateServiceAuthorityToken(token, time.Now()); err != nil {
		return "", err
	}

	// 刷新成功后写入缓存。
	p.token = strings.TrimSpace(token.Token)
	p.expiresAt = token.ExpiresAt
	// 返回刚写入缓存的 token。
	return p.token, nil
}

func (p *CachedServiceAuthorityProvider) isCachedTokenUsableLocked() bool {
	// 空 token 不能复用，必须重新 fetch。
	if strings.TrimSpace(p.token) == "" {
		return false
	}
	// 零值过期时间表示永久有效，可以一直复用。
	if p.expiresAt.IsZero() {
		return true
	}
	// 当前时间加刷新窗口仍早于过期时间时，说明 token 可继续使用。
	return time.Now().UTC().Add(p.refreshBefore).Before(p.expiresAt)
}

// NewServiceAuthorityToken 根据 auth 服务返回的 token 和 expired 字符串构造统一 token。
func NewServiceAuthorityToken(token string, expired string) (*ServiceAuthorityToken, error) {
	// 先解析 expired，允许永久有效或空值。
	expiresAt, err := ParseServiceAuthorityExpiresAt(expired)
	if err != nil {
		return nil, err
	}
	// 返回标准 token 结构，调用方无需关心 auth proto 的具体字段名。
	return &ServiceAuthorityToken{
		Token:     strings.TrimSpace(token),
		ExpiresAt: expiresAt,
	}, nil
}

// ParseServiceAuthorityExpiresAt 解析 auth 服务 SessionToken.expired 字段。
func ParseServiceAuthorityExpiresAt(value string) (time.Time, error) {
	// 目标链路要求 service token 必须有明确过期时间，便于调用方覆盖和动态轮换。
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrServiceAuthorityTokenExpiresAtMissing
	}
	// auth 服务当前使用 RFC3339 输出过期时间。
	expiresAt, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse service authority expires_at: %w", err)
	}
	// 统一转 UTC，避免比较时受本地时区影响。
	return expiresAt.UTC(), nil
}

func validateServiceAuthorityToken(token *ServiceAuthorityToken, now time.Time) error {
	// fetch 返回 nil 说明没有拿到有效 token。
	if token == nil {
		return ErrServiceAuthorityTokenMissing
	}
	// token 为空时不能作为服务身份凭证。
	if strings.TrimSpace(token.Token) == "" {
		return ErrServiceAuthorityTokenMissing
	}
	// 非零过期时间必须晚于当前时间。
	if token.ExpiresAt.IsZero() {
		return ErrServiceAuthorityTokenExpiresAtMissing
	}
	// 过期时间必须晚于当前时间。
	if !token.ExpiresAt.After(now.UTC()) {
		return ErrServiceAuthorityTokenExpired
	}
	// 校验通过后允许写入缓存。
	return nil
}

func parseServiceAuthorityRefreshBefore(value string) (time.Duration, error) {
	// 未配置时使用默认刷新窗口。
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultServiceAuthorityRefreshBefore, nil
	}
	// 使用标准 duration 解析，保持与已有 clock_skew 配置一致。
	refreshBefore, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse service authority refresh_before: %w", err)
	}
	// 非正数没有刷新意义，目标链路直接拒绝错误配置。
	if refreshBefore <= 0 {
		return 0, fmt.Errorf("parse service authority refresh_before: value must be positive")
	}
	// 返回调用方配置的刷新窗口。
	return refreshBefore, nil
}
