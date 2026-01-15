// Package consul 提供基于 Consul 的服务注册与服务发现实现。
package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	micro "github.com/fireflycore/go-micro/registry"
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

const (
	// consulMetaKeyNode 在 Consul Service.Meta 中保存 ServiceNode 的 JSON。
	consulMetaKeyNode = "node"
	// consulMetaKeyAppId 在 Consul Service.Meta 中保存 appId，方便排障与检索。
	consulMetaKeyAppId = "app_id"
	// consulMetaKeyEnv 在 Consul Service.Meta 中保存 env。
	consulMetaKeyEnv = "env"
	// consulMetaKeyVersion 在 Consul Service.Meta 中保存 version。
	consulMetaKeyVersion = "version"
)

// RegisterInstance 基于 Consul 的服务注册实例。
//
// 设计要点：
// - 使用 Consul Agent 的 TTL Check 作为“租约/心跳”能力；
// - Install 时注册一个服务实例并创建 TTL check；
// - SustainLease 周期性 UpdateTTL，维持健康状态；
// - 当心跳失败时按 conf.MaxRetry 做重试，并尝试重注册以恢复服务可见性。
type RegisterInstance struct {
	// ctx/cancel 控制注册实例生命周期：
	// - SustainLease 会阻塞运行，收到 ctx.Done() 后退出
	// - Uninstall() 调用 cancel() 并注销服务
	ctx    context.Context
	cancel context.CancelFunc

	// client 为外部注入的 Consul 客户端
	client *api.Client

	meta *micro.Meta
	conf *micro.ServiceConf

	// 当前已重试次数
	retryCount uint32
	// 重试前的回调
	retryBefore func()
	// 重试成功后的回调
	retryAfter func()

	log *zap.Logger

	// leaseId 作为该实例的“租约标识”，用于保持与 etcd 实现一致的 LeaseId 语义。
	// 注意：Consul 并不提供与 etcd leaseId 完全等价的概念，这里使用本地生成的稳定标识。
	leaseId int
	// serviceId 为本实例在 Consul 中的唯一服务 Id。
	serviceId string
	// checkId 为 TTL check 的 Id；Consul 默认以 "service:<serviceId>" 作为服务级 check Id。
	checkId string

	// lastNode 缓存最后一次注册的服务节点信息，用于重注册时复用。
	lastNode *micro.ServiceNode
}

// NewRegister 创建基于 Consul 的服务注册实例。
func NewRegister(client *api.Client, meta *micro.Meta, conf *micro.ServiceConf) (micro.Register, error) {
	if client == nil {
		return nil, fmt.Errorf(micro.ErrClientIsNil, "consul")
	}
	if meta == nil {
		return nil, micro.ErrServiceMetaIsNil
	}
	if conf == nil {
		return nil, micro.ErrServiceConfIsNil
	}
	conf.Bootstrap()

	ctx, cancel := context.WithCancel(context.Background())

	leaseId := int(time.Now().UnixNano())
	if leaseId == 0 {
		leaseId = 1
	}

	return &RegisterInstance{
		ctx:    ctx,
		cancel: cancel,

		client: client,
		meta:   meta,
		conf:   conf,

		leaseId: leaseId,
	}, nil
}

// Install 将服务节点注册到 Consul：
// - 补齐节点运行时信息（Meta/Kernel/Network/RunDate/LeaseId）
// - 通过 Consul Agent 注册服务并创建 TTL check
func (s *RegisterInstance) Install(service *micro.ServiceNode) error {
	if service == nil {
		return micro.ErrServiceNodeIsNil
	}

	if s.meta.AppId == "" {
		return fmt.Errorf("service meta appId 为空")
	}
	if s.meta.Env == "" {
		return fmt.Errorf("service meta env 为空")
	}

	service.Meta = s.meta
	service.Kernel = s.conf.Kernel
	service.Network = s.conf.Network
	service.LeaseId = s.leaseId
	service.RunDate = time.Now().Format(time.DateTime)

	s.lastNode = service

	if s.serviceId == "" {
		s.serviceId = s.newServiceId()
		s.checkId = "service:" + s.serviceId
	}

	return s.register()
}

// Uninstall 注销当前注册的服务实例并停止心跳。
func (s *RegisterInstance) Uninstall() {
	s.cancel()

	if s.serviceId == "" {
		return
	}

	_ = s.client.Agent().ServiceDeregister(s.serviceId)
}

// SustainLease 维持 TTL 心跳，直到 Uninstall 或调用方主动取消。
func (s *RegisterInstance) SustainLease() {
	interval := time.Duration(s.conf.TTL) * time.Second / 2
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.heartbeat(); err != nil {
				if !s.retryHeartbeat() {
					return
				}
			} else if s.retryCount != 0 {
				s.retryCount = 0
			}
		}
	}
}

// WithRetryBefore 设置重试前回调。
func (s *RegisterInstance) WithRetryBefore(handle func()) {
	s.retryBefore = handle
}

// WithRetryAfter 设置重试成功后的回调。
func (s *RegisterInstance) WithRetryAfter(handle func()) {
	s.retryAfter = handle
}

// WithLog 设置内部日志输出回调。
func (s *RegisterInstance) WithLog(log *zap.Logger) {
	s.log = log
}

func (s *RegisterInstance) newServiceId() string {
	ns := s.conf.Namespace
	env := s.meta.Env
	appId := s.meta.AppId
	return fmt.Sprintf("%s-%s-%s-%d-%s", ns, env, appId, s.leaseId, uuid.NewString())
}

func (s *RegisterInstance) serviceName() string {
	return fmt.Sprintf("%s-%s", s.conf.Namespace, s.meta.Env)
}

// register 执行实际的 Consul 注册动作，并把 TTL check 标记为 passing。
func (s *RegisterInstance) register() error {
	if s.lastNode == nil {
		return nil
	}

	b, err := json.Marshal(s.lastNode)
	if err != nil {
		return err
	}

	registration := &api.AgentServiceRegistration{
		ID:      s.serviceId,
		Name:    s.serviceName(),
		Address: s.conf.Network.Internal,
		Port:    0,
		Tags: []string{
			s.meta.AppId,
			s.meta.Version,
		},
		Meta: map[string]string{
			consulMetaKeyNode:    string(b),
			consulMetaKeyAppId:   s.meta.AppId,
			consulMetaKeyEnv:     s.meta.Env,
			consulMetaKeyVersion: s.meta.Version,
		},
		Check: &api.AgentServiceCheck{
			TTL:                            fmt.Sprintf("%ds", s.conf.TTL),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", s.conf.TTL*3),
		},
	}

	if err := s.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	return s.client.Agent().UpdateTTL(s.checkId, "ok", api.HealthPassing)
}

// heartbeat 通过 UpdateTTL 续命 TTL check。
func (s *RegisterInstance) heartbeat() error {
	if s.checkId == "" {
		return fmt.Errorf("consul checkId is null")
	}
	return s.client.Agent().UpdateTTL(s.checkId, "ok", api.HealthPassing)
}

// retryHeartbeat 当心跳失败时进行退避重试，并尝试重注册服务以恢复可用性。
func (s *RegisterInstance) retryHeartbeat() bool {
	for s.retryCount < s.conf.MaxRetry {
		if s.retryBefore != nil {
			s.retryBefore()
		}

		timer := time.NewTimer(time.Duration(s.conf.TTL) * 5 * time.Second)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}

		s.retryCount++

		if s.log != nil {
			s.log.Info("consul retry heartbeat", zap.Uint32("retryCount", s.retryCount), zap.Uint32("maxRetry", s.conf.MaxRetry))
		}

		if err := s.register(); err != nil {
			if s.log != nil {
				s.log.Error("consul re-register failed", zap.Error(err))
			}
			continue
		}

		if s.retryAfter != nil {
			s.retryAfter()
		}

		s.retryCount = 0
		return true
	}

	return false
}
