package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	"github.com/lhdhtrc/func-go/array"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// DiscoverInstance 服务发现实例
// 负责服务的注册、发现和监控
type DiscoverInstance struct {
	meta   *micro.Meta        // 服务元数据信息
	config *micro.ServiceConf // 服务配置信息
	client *clientv3.Client   // etcd客户端实例

	ctx    context.Context    // 上下文，用于控制生命周期
	cancel context.CancelFunc // 取消函数，用于停止监控

	log func(level logger.LogLevel, message string) // 日志记录函数

	method  micro.ServiceMethod   // 服务方法映射表 (method -> appId)
	service micro.ServiceDiscover // 服务发现数据 (appId -> []ServiceNode)

	mu sync.RWMutex
}

// NewDiscover 创建服务发现实例
// 参数:
//   - client: etcd客户端实例
//   - meta: 服务元数据信息
//   - config: 服务配置信息
//
// 返回:
//   - *DiscoverInstance: 服务发现实例
//   - error: 错误信息
func NewDiscover(client *clientv3.Client, meta *micro.Meta, config *micro.ServiceConf) (*DiscoverInstance, error) {
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

	// 创建可取消的上下文，用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())

	// 初始化服务发现实例
	instance := &DiscoverInstance{
		ctx:  ctx,
		meta: meta,

		cancel:  cancel,
		client:  client,
		config:  config,
		method:  make(micro.ServiceMethod),
		service: make(micro.ServiceDiscover),
	}

	// 执行引导初始化
	err := instance.bootstrap()

	return instance, err
}

// GetService 根据服务方法名获取对应的服务节点列表
// 参数:
//   - sm: 服务方法名
//
// 返回:
//   - []*micro.ServiceNode: 服务节点列表
//   - error: 错误信息，当服务方法不存在时返回错误
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
	// 返回 slice 的副本，避免调用方持有内部切片导致并发读写风险；节点指针本身仍共享。
	out := append([]*micro.ServiceNode(nil), nodes...)
	s.mu.RUnlock()
	return out, nil
}

// Watcher 启动服务发现监控
// 该方法会阻塞执行，持续监控etcd中的服务变化
// 通常在单独的goroutine中调用
func (s *DiscoverInstance) Watcher() {
	// 创建etcd监听器，监控指定命名空间和环境下的所有键值变化
	watchKey := fmt.Sprintf("%s/%s", s.config.Namespace, s.meta.Env)
	wc := s.client.Watch(s.ctx, watchKey, clientv3.WithPrefix(), clientv3.WithPrevKV())

	// 持续处理监控事件
	for v := range wc {
		for _, e := range v.Events {
			// 将etcd事件适配为服务发现事件
			s.adapter(e)
		}
	}
}

// Unwatch 停止服务发现监控并释放资源
// 调用此方法会取消上下文，停止所有的监控goroutine
func (s *DiscoverInstance) Unwatch() {
	s.cancel()
}

// WithLog 设置日志记录函数
// 参数:
//   - handle: 日志处理函数，接收日志级别和消息内容
func (s *DiscoverInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	s.log = handle
}

// bootstrap 初始化引导
// 从etcd中加载现有的服务注册信息，构建初始的服务发现数据
// 返回:
//   - error: 初始化过程中发生的错误
func (s *DiscoverInstance) bootstrap() error {
	// 从etcd获取指定命名空间、环境下的所有键值对
	res, err := s.client.Get(s.ctx, fmt.Sprintf("%s/%s", s.config.Namespace, s.meta.Env), clientv3.WithPrefix())
	if err != nil {
		return err
	}

	// 遍历所有获取到的键值对
	for _, item := range res.Kvs {
		var val micro.ServiceNode
		// 反序列化服务节点信息
		if err = json.Unmarshal(item.Value, &val); err == nil {
			if val.Meta == nil || val.Meta.AppId == "" || val.Meta.Env == "" {
				continue
			}
			appId := val.Meta.AppId

			s.mu.Lock()
			val.ParseMethod(s.method)
			s.upsertNodeLocked(appId, &val)
			s.mu.Unlock()
		}
	}

	// 记录初始化完成日志
	if s.log != nil {
		s.log(logger.Info, fmt.Sprintf("Bootstrap completed, discovered %d services", len(s.service)))
	}

	return nil
}

// adapter 服务发现适配器
// 将etcd的原始事件转换为服务发现内部事件
// 参数:
//   - e: etcd事件，包含事件类型和键值信息
func (s *DiscoverInstance) adapter(e *clientv3.Event) {
	var (
		tv []byte
	)

	// 确定要处理的值数据，删除事件使用前一个值，其他事件使用当前值
	if e.PrevKv != nil {
		tv = e.PrevKv.Value
	} else {
		tv = e.Kv.Value
	}

	// 反序列化服务节点信息
	var val micro.ServiceNode
	if err := json.Unmarshal(tv, &val); err != nil {
		// 记录反序列化错误
		if s.log != nil {
			s.log(logger.Error, fmt.Sprintf("Failed to unmarshal service node: %s", err.Error()))
		}
		return
	}
	if val.Meta == nil || val.Meta.AppId == "" || val.Meta.Env == "" {
		return
	}

	s.mu.Lock()
	val.ParseMethod(s.method)
	switch e.Type {
	case clientv3.EventTypePut: // 新增或更新服务节点
		s.upsertNodeLocked(val.Meta.AppId, &val)
	case clientv3.EventTypeDelete: // 删除服务节点
		s.deleteNodeLocked(val.Meta.AppId, &val)
	}
	s.mu.Unlock()
}

func (s *DiscoverInstance) upsertNodeLocked(appId string, newNode *micro.ServiceNode) {
	nodes := s.service[appId]
	nodes = array.Filter(nodes, func(_ int, item *micro.ServiceNode) bool {
		return item.LeaseId != newNode.LeaseId
	})
	s.service[appId] = append([]*micro.ServiceNode{newNode}, nodes...)

	if s.log != nil {
		s.log(logger.Info, fmt.Sprintf("Service updated: %s, leaseId: %d, nodes count: %d", appId, newNode.LeaseId, len(s.service[appId])))
	}
}

func (s *DiscoverInstance) deleteNodeLocked(appId string, removedNode *micro.ServiceNode) {
	originalCount := len(s.service[appId])
	s.service[appId] = array.Filter(s.service[appId], func(_ int, item *micro.ServiceNode) bool {
		return item.LeaseId != removedNode.LeaseId
	})

	if s.log != nil {
		remainingCount := len(s.service[appId])
		if originalCount != remainingCount {
			s.log(logger.Info, fmt.Sprintf("Service removed: %s, leaseId: %d, nodes count: %d -> %d", appId, removedNode.LeaseId, originalCount, remainingCount))
		}
	}

	if len(s.service[appId]) == 0 {
		delete(s.service, appId)
		for sm, owner := range s.method {
			if owner == appId {
				delete(s.method, sm)
			}
		}
		if s.log != nil {
			s.log(logger.Info, fmt.Sprintf("Service %s has no nodes, removed from discovery", appId))
		}
	}
}
