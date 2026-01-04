// Package consul 提供基于 consul 的服务注册与服务发现实现。
package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	"github.com/hashicorp/consul/api"
)

// DiscoverInstance 服务发现实例
type DiscoverInstance struct {
	meta   *micro.Meta
	config *micro.ServiceConf
	client *api.Client

	ctx    context.Context
	cancel context.CancelFunc

	log func(level logger.LogLevel, message string)

	method  micro.ServiceMethod
	service micro.ServiceDiscover

	watchers map[string]context.CancelFunc // 管理每个服务的 watcher

	mu sync.RWMutex
}

// NewDiscover 创建服务发现实例
func NewDiscover(client *api.Client, meta *micro.Meta, config *micro.ServiceConf) (micro.Discovery, error) {
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
	if config.Network == nil {
		config.Network = &micro.Network{}
	}
	if config.Kernel == nil {
		config.Kernel = &micro.Kernel{}
	}

	ctx, cancel := context.WithCancel(context.Background())

	instance := &DiscoverInstance{
		ctx:  ctx,
		meta: meta,

		cancel:  cancel,
		client:  client,
		config:  config,
		method:  make(micro.ServiceMethod),
		service: make(micro.ServiceDiscover),

		watchers: make(map[string]context.CancelFunc),
	}

	// 执行引导初始化
	err := instance.bootstrap()

	return instance, err
}

// GetService 根据服务方法名获取对应的服务节点列表
func (s *DiscoverInstance) GetService(sm string) ([]*micro.ServiceNode, error) {
	s.mu.RLock()
	appId, ok := s.method[sm]
	if !ok {
		s.mu.RUnlock()
		return nil, micro.ErrServiceMethodNotExists
	}

	nodes, ok := s.service[appId]
	if !ok {
		s.mu.RUnlock()
		return nil, micro.ErrServiceNodeNotExists
	}
	out := append([]*micro.ServiceNode(nil), nodes...)
	s.mu.RUnlock()
	return out, nil
}

// Watcher 启动服务发现监控
func (s *DiscoverInstance) Watcher() {
	// 1. 启动 Catalog Watcher，监控服务列表变化
	go s.watchCatalog()
}

// Unwatch 停止服务发现监控并释放资源
func (s *DiscoverInstance) Unwatch() {
	s.cancel()
}

// WithLog 设置日志记录函数
func (s *DiscoverInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	s.log = handle
}

// bootstrap 初始化引导
func (s *DiscoverInstance) bootstrap() error {
	// 获取所有服务
	services, _, err := s.client.Catalog().Services(nil)
	if err != nil {
		return err
	}

	for name, tags := range services {
		if s.shouldWatch(name, tags) {
			// 同步获取一次实例
			s.syncService(name)
		}
	}
	return nil
}

// shouldWatch 判断是否需要监控该服务
func (s *DiscoverInstance) shouldWatch(name string, tags []string) bool {
	// 简单过滤：检查 tags 是否包含 namespace 和 env
	nsTag := fmt.Sprintf("namespace=%s", s.config.Namespace)
	envTag := fmt.Sprintf("env=%s", s.meta.Env)

	hasNs := false
	hasEnv := false

	for _, t := range tags {
		if t == nsTag {
			hasNs = true
		}
		if t == envTag {
			hasEnv = true
		}
	}
	return hasNs && hasEnv
}

// watchCatalog 监控服务列表变化
func (s *DiscoverInstance) watchCatalog() {
	var lastIndex uint64
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		opts := &api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  time.Second * 30,
		}
		services, meta, err := s.client.Catalog().Services(opts)
		if err != nil {
			if s.log != nil {
				s.log(logger.Error, fmt.Sprintf("watch catalog failed: %v", err))
			}
			time.Sleep(time.Second * 5)
			continue
		}

		if meta != nil {
			lastIndex = meta.LastIndex
		}

		// 检查现有服务，启动或停止 watcher
		currentServices := make(map[string]bool)
		for name, tags := range services {
			if s.shouldWatch(name, tags) {
				currentServices[name] = true
				s.mu.Lock()
				if _, ok := s.watchers[name]; !ok {
					// 新发现的服务，启动 watcher
					ctx, cancel := context.WithCancel(s.ctx)
					s.watchers[name] = cancel
					go s.watchService(ctx, name)
				}
				s.mu.Unlock()
			}
		}

		// 清理不再存在的服务
		s.mu.Lock()
		for name, cancel := range s.watchers {
			if !currentServices[name] {
				cancel()
				delete(s.watchers, name)
				delete(s.service, name)
				// 清理 methods 表中该服务的条目 (比较复杂，需要遍历 methods)
				// 简单起见，这里暂不清理 methods，因为可能有残留，但 GetService 会检查 s.service[appId]
			}
		}
		s.mu.Unlock()
	}
}

// watchService 监控单个服务的实例变化
func (s *DiscoverInstance) watchService(ctx context.Context, serviceName string) {
	var lastIndex uint64
	tag := fmt.Sprintf("env=%s", s.meta.Env)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		opts := &api.QueryOptions{
			WaitIndex: lastIndex,
			WaitTime:  time.Second * 30,
			Filter:    fmt.Sprintf("Service == \"%s\" and \"%s\" in Tags", serviceName, tag),
		}

		// 使用 Health().Service 获取健康实例
		entries, meta, err := s.client.Health().Service(serviceName, "", true, opts)
		if err != nil {
			if s.log != nil {
				s.log(logger.Error, fmt.Sprintf("watch service %s failed: %v", serviceName, err))
			}
			time.Sleep(time.Second * 5)
			continue
		}

		if meta != nil {
			lastIndex = meta.LastIndex
		}

		s.updateServiceNodes(serviceName, entries)
	}
}

// syncService 同步获取服务实例（非阻塞）
func (s *DiscoverInstance) syncService(serviceName string) {
	tag := fmt.Sprintf("env=%s", s.meta.Env)
	entries, _, err := s.client.Health().Service(serviceName, tag, true, &api.QueryOptions{})
	if err != nil {
		return
	}
	s.updateServiceNodes(serviceName, entries)
}

func (s *DiscoverInstance) updateServiceNodes(serviceName string, entries []*api.ServiceEntry) {
	var nodes []*micro.ServiceNode

	for _, entry := range entries {
		// 从 Meta["payload"] 解析 ServiceNode
		if payloadStr, ok := entry.Service.Meta["payload"]; ok {
			var node micro.ServiceNode
			if err := json.Unmarshal([]byte(payloadStr), &node); err == nil {
				nodes = append(nodes, &node)
			}
		}
	}

	s.mu.Lock()
	if len(nodes) > 0 {
		s.service[serviceName] = nodes
		// 更新 methods 映射
		for _, node := range nodes {
			node.ParseMethod(s.method)
		}
		if s.log != nil {
			s.log(logger.Info, fmt.Sprintf("Service updated: %s, nodes count: %d", serviceName, len(nodes)))
		}
	} else {
		// 如果没有实例，删除服务
		delete(s.service, serviceName)
	}
	s.mu.Unlock()
}
