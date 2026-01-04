// Package etcd 提供基于 etcd 的服务注册与服务发现实现。
package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// NewRegister 创建基于 etcd 的服务注册实例。
func NewRegister(client *clientv3.Client, meta *micro.Meta, config *micro.ServiceConf) (*RegisterInstance, error) {
	if client == nil {
		return nil, errors.New("etcd client is nil")
	}
	if config == nil {
		return nil, errors.New("service config is nil")
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
}

// Install 将服务节点写入注册中心，并绑定到当前 lease。
func (s *RegisterInstance) Install(service *micro.ServiceNode) error {
	if service == nil {
		return errors.New("service node is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

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
		return errors.New("meta.app_id is empty")
	}
	if effectiveMeta.Env == "" {
		return errors.New("meta.env is empty")
	}

	service.Meta = effectiveMeta
	service.Kernel = s.config.Kernel
	service.Network = s.config.Network
	service.LeaseId = int(s.lease)
	service.RunDate = time.Now().Format(time.DateTime)

	val, _ := json.Marshal(service)

	_, err := s.client.Put(ctx, fmt.Sprintf("%s/%s/%s/%d", s.config.Namespace, effectiveMeta.Env, effectiveMeta.AppId, s.lease), string(val), clientv3.WithLease(s.lease))
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

		if err := s.initLease(); err != nil {
			continue
		}
		if s.retryAfter != nil {
			s.retryAfter()
		}
		return true
	}

	return false
}
