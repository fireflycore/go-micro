package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	micro "github.com/fireflycore/go-micro/registry"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// DiscoverInstance 基于 Consul 的服务发现实例。
//
// 设计要点：
// - 通过 Consul Health API 获取指定 serviceName（namespace-env）的实例列表；
// - 将每个实例 Meta 中保存的 ServiceNode(JSON) 反序列化并构建本地缓存：
//   - service: appId -> 节点列表
//   - method:  method -> appId（派生索引，用于 GetService 快速定位）
//
// - Watcher 使用阻塞查询（WaitIndex）持续拉取变更后的快照，并重建本地缓存。
type DiscoverInstance struct {
	// mu 保护 method/service 两个内存索引：
	// - GetService 走读锁
	// - rebuild 写入时走写锁
	mu sync.RWMutex

	// ctx/cancel 控制发现实例生命周期：
	// - Watcher 会阻塞运行，收到 ctx.Done() 后退出
	// - Unwatch() 调用 cancel() 触发退出
	ctx    context.Context
	cancel context.CancelFunc

	// client 为外部注入的 Consul 客户端
	client *api.Client

	meta *micro.Meta
	conf *micro.ServiceConf

	// service 是发现的“主表”：appId -> 节点列表
	// method 是 service 的“派生索引”：method -> appId
	method  micro.ServiceMethod
	service micro.ServiceDiscover

	log *zap.Logger

	// waitIndex 用于 Consul 阻塞查询的游标（类似 etcd revision）。
	// - bootstrap() 阶段：使用响应头的 X-Consul-Index 初始化
	// - Watcher() 阶段：每次请求把 WaitIndex 带上，实现“有变更才返回”的阻塞效果
	waitIndex uint64
}

// NewDiscover 创建基于 Consul 的服务发现实例。
// 参数:
//   - client: Consul 客户端
//   - meta: 服务元数据信息
//   - config: 服务配置信息
//
// 返回:
//   - micro.Discovery: 服务发现接口实现
//   - error:           错误信息
func NewDiscover(client *api.Client, meta *micro.Meta, conf *micro.ServiceConf) (micro.Discovery, error) {
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

	// 创建可取消的上下文，用于优雅退出；cancel 会被 Unwatch 调用
	ctx, cancel := context.WithCancel(context.Background())

	// 初始化服务发现实例。
	instance := &DiscoverInstance{
		ctx:    ctx,
		cancel: cancel,
		client: client,

		meta: meta,
		conf: conf,

		method:  make(micro.ServiceMethod),
		service: make(micro.ServiceDiscover),
	}

	if err := instance.bootstrap(); err != nil {
		return nil, err
	}

	return instance, nil
}

// GetService 根据 gRPC 方法名获取对应的服务节点列表。
// 参数:
//   - sm: 服务方法名
//
// 返回:
//   - []*micro.ServiceNode: 服务节点列表
//   - error: 错误信息，当服务方法不存在时返回错误
func (s *DiscoverInstance) GetService(sm string) ([]*micro.ServiceNode, error) {
	// 读锁：允许并发读取，但禁止写入并发
	s.mu.RLock()

	// 通过方法名定位到所属 appId
	appId, ok := s.method[sm]
	if !ok {
		s.mu.RUnlock()
		return nil, micro.ErrServiceMethodNotExists
	}

	// 根据 appId 取出节点列表
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

// Watcher 启动阻塞监听并持续刷新本地缓存。
// 该方法会阻塞执行，通常在单独的 goroutine 中调用。
func (s *DiscoverInstance) Watcher() {
	// Consul 的 watch 通过阻塞查询实现：
	// - WaitIndex: 指定“从哪个索引之后开始等待变化”
	// - WaitTime:  最大阻塞时长（到期后即使无变化也会返回一次）
	for {
		select {
		case <-s.ctx.Done():
			// Unwatch() 调用 cancel 后，退出 Watcher
			return
		default:
			// 不阻塞：继续发起阻塞查询
		}

		// 每次循环都构造新的 QueryOptions：
		// - WaitIndex 使用上一次响应的 LastIndex，衔接连续的阻塞查询
		// - WaitTime 兜底避免永远阻塞（同时也便于 Unwatch 后最迟在 WaitTime 内退出）
		opts := &api.QueryOptions{
			WaitIndex: s.waitIndex,
			WaitTime:  30 * time.Second,
		}

		// passingOnly=true：只返回健康节点（TTL 心跳未通过的实例不会出现在列表中）
		// WithContext(s.ctx)：让 Unwatch() 能中断阻塞请求，避免 goroutine 长时间挂起
		entries, meta, err := s.client.Health().Service(s.serviceName(), "", true, opts.WithContext(s.ctx))
		if err != nil {
			// ctx 取消时，Consul 客户端通常会返回 context canceled；此处直接退出
			if s.ctx.Err() != nil {
				return
			}
			// 其他错误（网络抖动/Consul 不可用等）做小退避，避免空转打满 CPU/网络
			timer := time.NewTimer(200 * time.Millisecond)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}

		// 更新游标：下一次阻塞查询从最新的索引之后开始等待变化
		if meta != nil && meta.LastIndex != 0 {
			s.waitIndex = meta.LastIndex
		}

		// 用最新快照重建索引（主表 + 派生索引）
		s.rebuild(entries)
	}
}

// Unwatch 停止监听并释放相关资源。
func (s *DiscoverInstance) Unwatch() {
	// 调用 cancel 触发 ctx.Done()，Watcher 会尽快退出
	s.cancel()
}

// WithLog 设置内部日志回调。
func (s *DiscoverInstance) WithLog(log *zap.Logger) {
	s.log = log
}

// serviceName 将 Namespace/Env 映射到 Consul service 名称空间。
// 约定：
// - 注册侧使用同样的规则把实例注册为 serviceName；
// - 发现侧按该名称查询所有实例，再按 Meta 中的 appId 聚合成主表。
func (s *DiscoverInstance) serviceName() string {
	return fmt.Sprintf("%s-%s", s.conf.Namespace, s.meta.Env)
}

// bootstrap 从 Consul 拉取一次快照，构建初始缓存。
func (s *DiscoverInstance) bootstrap() error {
	// 首次快照不做阻塞查询，直接用 nil QueryOptions 拉取当前服务列表
	entries, meta, err := s.client.Health().Service(s.serviceName(), "", true, nil)
	if err != nil {
		return err
	}
	// 保存当前索引作为后续 Watcher 的 WaitIndex 起点
	if meta != nil && meta.LastIndex != 0 {
		s.waitIndex = meta.LastIndex
	}

	// 通过快照构建本地缓存
	s.rebuild(entries)

	// 记录初始化完成日志（如果上层设置了日志回调）
	if s.log != nil {
		s.log.Info("bootstrap completed", zap.Int("services", len(s.service)))
	}

	return nil
}

// rebuild 使用 Consul 返回的实例快照重建本地索引。
func (s *DiscoverInstance) rebuild(entries []*api.ServiceEntry) {
	// 用新的 map 先构建完整快照，再一次性替换旧索引，避免“构建一半”时被 GetService 读到
	nextService := make(micro.ServiceDiscover)
	nextMethod := make(micro.ServiceMethod)

	// 第一步：把 Consul 返回的实例列表按 appId 聚合到 nextService（主表）
	for _, entry := range entries {
		// 防御空指针：Consul 响应中可能出现 nil entry 或缺少 Service 字段
		if entry == nil || entry.Service == nil {
			continue
		}
		// 本实现约定把 ServiceNode(JSON) 放在 Service.Meta 中；Meta 为空则跳过
		if entry.Service.Meta == nil {
			continue
		}

		// 进一步过滤非本 SDK 写入的数据：
		// - 注册侧会写入 env，缺失或不匹配的直接跳过
		// - 这样即使同名 service 下混入其他注册方式的数据，也不会污染本地缓存
		env, ok := entry.Service.Meta[consulMetaKeyEnv]
		if !ok || env != s.meta.Env {
			continue
		}

		// 从 Meta 中读取注册侧写入的 node（ServiceNode JSON）
		raw, ok := entry.Service.Meta[consulMetaKeyNode]
		if !ok || raw == "" {
			continue
		}

		// 反序列化为 ServiceNode；使用值类型接收，便于 append 时取地址
		var node micro.ServiceNode
		if err := json.Unmarshal([]byte(raw), &node); err != nil {
			// JSON 解析失败通常意味着注册侧写入异常或 Consul 数据被污染；记录日志并跳过
			if s.log != nil {
				s.log.Error("failed to unmarshal service node", zap.Error(err))
			}
			continue
		}

		// 防御脏数据：Meta 不完整会导致后续索引构建异常
		if node.Meta == nil || node.Meta.AppId == "" || node.Meta.Env == "" {
			continue
		}
		// 追加到该 appId 下的节点列表
		nodes := nextService[node.Meta.AppId]
		nodes = append(nodes, &node)
		nextService[node.Meta.AppId] = nodes
	}

	// 第二步：由 nextService 推导 nextMethod（派生索引），保证两者一致
	for appId, nodes := range nextService {
		for _, node := range nodes {
			// 防御空指针：理论上不会出现，但避免未来改动引入风险
			if node == nil {
				continue
			}
			// 一个节点的 Methods 是 method -> true 的集合；把它们归属到 appId
			for sm := range node.Methods {
				nextMethod[sm] = appId
			}
		}
	}

	// 第三步：一次性替换旧索引，保证 GetService 读到的是同一时刻的快照
	s.mu.Lock()
	s.service = nextService
	s.method = nextMethod
	s.mu.Unlock()
}
