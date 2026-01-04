// Package etcd 提供基于 etcd 的服务注册与服务发现实现。
package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// RegisterInstance 表示一次注册会话，使用 etcd lease 维持存活。
type RegisterInstance struct {
	meta   *micro.Meta
	config *micro.ServiceConf
	client *clientv3.Client
	lease  clientv3.LeaseID

	ctx    context.Context
	cancel context.CancelFunc

	retryCount  uint32
	retryBefore func()
	retryAfter  func()
	log         func(level logger.LogLevel, message string)

	// lastNode 缓存最后一次注册的服务节点信息，用于断线重连
	lastNode *micro.ServiceNode
}

// NewRegister 创建基于 etcd 的服务注册实例。
func NewRegister(client *clientv3.Client, meta *micro.Meta, config *micro.ServiceConf) (*RegisterInstance, error) {
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

	instance := &RegisterInstance{
		ctx:  ctx,
		meta: meta,

		config: config,
		client: client,
		cancel: cancel,
	}
	err := instance.initLease()

	return instance, err
}

// Install 将服务节点写入注册中心，并绑定到当前 lease。
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
	service.LeaseId = int(s.lease)
	service.RunDate = time.Now().Format(time.DateTime)

	// 缓存节点信息，用于重试
	s.lastNode = service

	return s.register()
}

// register 执行实际的 etcd put 操作
func (s *RegisterInstance) register() error {
	if s.lastNode == nil {
		return nil
	}

	val, _ := json.Marshal(s.lastNode)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	_, err := s.client.Put(ctx, fmt.Sprintf("%s/%s/%s/%d", s.config.Namespace, s.lastNode.Meta.Env, s.lastNode.Meta.AppId, s.lease), string(val), clientv3.WithLease(s.lease))
	return err
}

// Uninstall 撤销 lease 并停止续约。
func (s *RegisterInstance) Uninstall() {
	defer s.cancel()
	_, _ = s.client.Revoke(context.Background(), s.lease)
	return
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

// initLease 初始化租约
func (s *RegisterInstance) initLease() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	grant, err := s.client.Grant(ctx, int64(s.config.TTL))
	if err != nil {
		return err
	}
	s.lease = grant.ID

	return nil
}

// SustainLease 持续续约，直到上下文被取消。
func (s *RegisterInstance) SustainLease() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		leaseCh, err := s.client.KeepAlive(s.ctx, s.lease)
		if err != nil || leaseCh == nil {
			if !s.retryLease() {
				return
			}
			continue
		}

		for {
			select {
			case <-s.ctx.Done():
				return
			case _, ok := <-leaseCh:
				if !ok {
					if !s.retryLease() {
						return
					}
					goto next
				}
				if s.retryCount != 0 {
					s.retryCount = 0
				}
			}
		}
	next:
	}
}

func (s *RegisterInstance) retryLease() bool {
	if s.config.MaxRetry == 0 {
		return false
	}

	for s.retryCount < s.config.MaxRetry {
		if s.retryBefore != nil {
			s.retryBefore()
		}

		timer := time.NewTimer(5 * time.Second)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}

		s.retryCount++
		if s.log != nil {
			s.log(logger.Info, fmt.Sprintf("etcd retry lease: %d/%d", s.retryCount, s.config.MaxRetry))
		}

		// 重新获取 lease
		if err := s.initLease(); err != nil {
			continue
		}

		// 获取新 lease 成功后，必须重新注册服务数据
		// 因为旧 lease 过期后，数据会被 etcd 删除
		if s.lastNode != nil {
			// 更新 leaseId
			s.lastNode.LeaseId = int(s.lease)
			if err := s.register(); err != nil {
				if s.log != nil {
					s.log(logger.Error, fmt.Sprintf("etcd re-register failed: %v", err))
				}
				continue
			}
		}

		if s.retryAfter != nil {
			s.retryAfter()
		}

		return true
	}

	return false
}
