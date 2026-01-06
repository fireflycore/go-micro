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

// RegisterInstance 基于 etcd 的服务注册实例
type RegisterInstance struct {
	ctx    context.Context
	cancel context.CancelFunc

	client *clientv3.Client
	lease  clientv3.LeaseID

	meta *micro.Meta
	conf *micro.ServiceConf

	// 当前已重试次数
	retryCount uint32
	// 重试前的回调
	retryBefore func()
	// 重试成功后的回调
	retryAfter func()

	log func(level logger.LogLevel, message string)

	// lastNode 缓存最后一次注册的服务节点信息，用于断线重连
	lastNode *micro.ServiceNode
}

// NewRegister 创建基于 etcd 的服务注册实例。
func NewRegister(client *clientv3.Client, meta *micro.Meta, conf *micro.ServiceConf) (micro.Register, error) {
	if client == nil {
		return nil, fmt.Errorf(micro.ErrClientIsNil, "etcd")
	}
	if meta == nil {
		return nil, micro.ErrServiceMetaIsNil
	}
	if conf == nil {
		return nil, micro.ErrServiceConfIsNil
	}
	conf.Bootstrap()

	ctx, cancel := context.WithCancel(context.Background())

	instance := &RegisterInstance{
		ctx:    ctx,
		cancel: cancel,

		client: client,

		meta: meta,
		conf: conf,
	}

	err := instance.initLease()
	if err != nil {
		return nil, err
	}

	return instance, nil
}

// Install 将服务节点写入注册中心，并绑定到当前 lease。
func (s *RegisterInstance) Install(service *micro.ServiceNode) error {
	if service == nil {
		return micro.ErrServiceNodeIsNil
	}

	service.Meta = s.meta
	service.Kernel = s.conf.Kernel
	service.Network = s.conf.Network
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

	val, err := json.Marshal(s.lastNode)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	_, err = s.client.Put(ctx, fmt.Sprintf("%s/%s/%s/%d", s.conf.Namespace, s.meta.Env, s.meta.AppId, s.lease), string(val), clientv3.WithLease(s.lease))
	return err
}

// Uninstall 撤销 lease 并停止续约。
func (s *RegisterInstance) Uninstall() {
	s.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.client.Revoke(ctx, s.lease)
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
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	grant, err := s.client.Grant(ctx, int64(s.conf.TTL))
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
	// 在次数未超限前循环重试
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
			s.log(logger.Info, fmt.Sprintf("etcd retry lease: %d/%d", s.retryCount, s.conf.MaxRetry))
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

		s.retryCount = 0
		return true
	}

	return false
}
