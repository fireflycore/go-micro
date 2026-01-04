// Package consul 提供基于 consul 的服务注册与服务发现实现。
package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	"github.com/hashicorp/consul/api"
)

// RegisterInstance 表示一次注册会话。
type RegisterInstance struct {
	meta   *micro.Meta
	config *micro.ServiceConf
	client *api.Client

	leaseId   int
	serviceId string
	checkId   string

	ctx    context.Context
	cancel context.CancelFunc

	retryCount  uint32
	retryBefore func()
	retryAfter  func()
	log         func(level logger.LogLevel, message string)

	// lastRegistration 缓存最后一次注册信息，用于断线重连
	lastRegistration *api.AgentServiceRegistration
}

// NewRegister 创建基于 consul 的服务注册实例。
func NewRegister(client *api.Client, meta *micro.Meta, config *micro.ServiceConf) (*RegisterInstance, error) {
	if client == nil {
		return nil, ErrClientIsNil
	}
	if config == nil {
		return nil, micro.ErrServiceConfigIsNil
	}
	if meta == nil {
		meta = &micro.Meta{}
	}
	if config.Namespace == "" {
		config.Namespace = "micro"
	}
	if config.TTL == 0 {
		config.TTL = 10
	}
	if config.Network == nil {
		config.Network = &micro.Network{}
	}
	if config.Kernel == nil {
		config.Kernel = &micro.Kernel{}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 生成一个随机的 LeaseId (模拟 etcd lease id，作为实例唯一标识的一部分)
	leaseId := rand.Intn(1000000000)

	instance := &RegisterInstance{
		ctx:  ctx,
		meta: meta,

		config:  config,
		client:  client,
		cancel:  cancel,
		leaseId: leaseId,
	}

	return instance, nil
}

// Install 将服务节点写入注册中心。
func (s *RegisterInstance) Install(service *micro.ServiceNode) error {
	if service == nil {
		return micro.ErrServiceNodeIsNil
	}

	if s.config.Kernel != nil && s.config.Kernel.Language == "" {
		s.config.Kernel.Language = "Golang"
	}

	effectiveMeta := s.meta
	if effectiveMeta == nil {
		effectiveMeta = &micro.Meta{}
	}
	if service.Meta != nil {
		if effectiveMeta.AppId == "" {
			effectiveMeta.AppId = service.Meta.AppId
		}
		if effectiveMeta.Env == "" {
			effectiveMeta.Env = service.Meta.Env
		}
		if effectiveMeta.Version == "" {
			effectiveMeta.Version = service.Meta.Version
		}
	}
	if effectiveMeta.AppId == "" {
		return micro.ErrMetaAppIdIsEmpty
	}
	if effectiveMeta.Env == "" {
		return micro.ErrMetaEnvIsEmpty
	}

	service.Meta = effectiveMeta
	service.Kernel = s.config.Kernel
	service.Network = s.config.Network
	service.LeaseId = s.leaseId
	service.RunDate = time.Now().Format(time.DateTime)

	// 构造服务ID，确保唯一性
	s.serviceId = fmt.Sprintf("%s-%d", effectiveMeta.AppId, s.leaseId)
	s.checkId = fmt.Sprintf("service:%s", s.serviceId)

	// 序列化 ServiceNode 存入 Meta
	payload, _ := json.Marshal(service)

	tags := []string{
		"micro",
		fmt.Sprintf("namespace=%s", s.config.Namespace),
		fmt.Sprintf("env=%s", effectiveMeta.Env),
	}

	registration := &api.AgentServiceRegistration{
		ID:      s.serviceId,
		Name:    effectiveMeta.AppId,
		Tags:    tags,
		Meta:    map[string]string{"payload": string(payload)},
		Address: service.Network.Internal, // 优先使用内网IP
		Check: &api.AgentServiceCheck{
			CheckID:                        s.checkId,
			TTL:                            fmt.Sprintf("%ds", s.config.TTL),
			DeregisterCriticalServiceAfter: fmt.Sprintf("%ds", s.config.TTL*3), // 3倍TTL后注销
		},
	}

	// 注册服务
	err := s.client.Agent().ServiceRegister(registration)
	if err != nil {
		return err
	}

	// 缓存注册信息
	s.lastRegistration = registration

	// 立即发送一次心跳，激活 Check
	return s.client.Agent().PassTTL(s.checkId, "initial heartbeat")
}

// Uninstall 注销服务。
func (s *RegisterInstance) Uninstall() {
	defer s.cancel()
	_ = s.client.Agent().ServiceDeregister(s.serviceId)
}

// WithLog 设置内部日志输出回调。
func (s *RegisterInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	s.log = handle
}

// WithRetryBefore 设置重试前回调。
func (s *RegisterInstance) WithRetryBefore(handle func()) {
	s.retryBefore = handle
}

// WithRetryAfter 设置重试成功后回调。
func (s *RegisterInstance) WithRetryAfter(handle func()) {
	s.retryAfter = handle
}

// SustainLease 持续发送心跳 (PassTTL)。
func (s *RegisterInstance) SustainLease() {
	ticker := time.NewTicker(time.Duration(s.config.TTL/2) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			err := s.client.Agent().PassTTL(s.checkId, "passing")
			if err != nil {
				if s.log != nil {
					s.log(logger.Error, fmt.Sprintf("consul heartbeat failed: %v", err))
				}
				// 尝试重注册
				if !s.retryRegister() {
					return
				}
			} else {
				// 重置重试计数
				if s.retryCount != 0 {
					s.retryCount = 0
				}
			}
		}
	}
}

// retryRegister 尝试重新注册服务
func (s *RegisterInstance) retryRegister() bool {
	if s.config.MaxRetry == 0 {
		return false
	}

	for s.retryCount < s.config.MaxRetry {
		if s.retryBefore != nil {
			s.retryBefore()
		}

		// 简单的退避策略
		time.Sleep(time.Second * 2)

		select {
		case <-s.ctx.Done():
			return false
		default:
		}

		s.retryCount++
		if s.log != nil {
			s.log(logger.Info, fmt.Sprintf("consul retry register: %d/%d", s.retryCount, s.config.MaxRetry))
		}

		// 重新注册逻辑
		if s.lastRegistration != nil {
			// 尝试重新注册
			err := s.client.Agent().ServiceRegister(s.lastRegistration)
			if err == nil {
				// 注册成功，发送心跳
				if err := s.client.Agent().PassTTL(s.checkId, "re-register heartbeat"); err == nil {
					if s.log != nil {
						s.log(logger.Info, "consul re-register success")
					}
					if s.retryAfter != nil {
						s.retryAfter()
					}
					return true
				}
			} else {
				if s.log != nil {
					s.log(logger.Error, fmt.Sprintf("consul re-register failed: %v", err))
				}
			}
		}

		return true // 继续尝试
	}
	return false
}
