package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DiscoverInstance 基于 Kubernetes ConfigMap 的服务发现实例。
//
// 本地索引模型对齐 etcd/consul：
// - service: appId -> 节点列表（主表）
// - method:  method -> appId（派生索引，用于 GetService 快速定位）
//
// Watcher 采用轮询方式刷新：
// - 使用 ConfigMap.metadata.resourceVersion 判断是否有变化；
// - 有变化时重建索引，保证 method 与 service 一致。
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

	client kubernetes.Interface

	// meta/conf 用于过滤环境与补齐默认值。
	meta *micro.Meta
	conf *micro.ServiceConf

	// service 是发现的“主表”：appId -> 节点列表
	// method 是 service 的“派生索引”：method -> appId
	method  micro.ServiceMethod
	service micro.ServiceDiscover

	// log 用于输出实现内部状态（对齐 etcd/consul 的模式）。
	log func(level logger.LogLevel, message string)

	// resourceVersion 是上一次读取到的 ConfigMap 版本号，用于判断是否需要刷新缓存。
	resourceVersion string
}

// NewDiscover 创建基于 Kubernetes 的服务发现实例。
func NewDiscover(client kubernetes.Interface, meta *micro.Meta, conf *micro.ServiceConf) (micro.Discovery, error) {
	if client == nil {
		return nil, fmt.Errorf(micro.ErrClientIsNil, "kubernetes")
	}
	if meta == nil {
		return nil, micro.ErrServiceMetaIsNil
	}
	if conf == nil {
		return nil, micro.ErrServiceConfIsNil
	}
	conf.Bootstrap()

	// ctx 用于控制 Watcher 的退出；Unwatch 会触发 cancel。
	ctx, cancel := context.WithCancel(context.Background())

	ins := &DiscoverInstance{
		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		meta:    meta,
		conf:    conf,
		method:  make(micro.ServiceMethod),
		service: make(micro.ServiceDiscover),
	}

	// 初始快照：保证 GetService 在 Watcher 启动前即可工作。
	if err := ins.bootstrap(); err != nil {
		return nil, err
	}

	return ins, nil
}

// GetService 根据 gRPC 方法名获取对应的服务节点列表。
func (s *DiscoverInstance) GetService(sm string) ([]*micro.ServiceNode, error) {
	// 读锁：允许并发读取，但禁止与写入并发。
	s.mu.RLock()

	// 通过方法名定位到所属 appId。
	appId, ok := s.method[sm]
	if !ok {
		s.mu.RUnlock()
		return nil, micro.ErrServiceMethodNotExists
	}

	// 根据 appId 取出节点列表。
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

// Watcher 启动轮询监听并持续刷新本地缓存。
// 该方法会阻塞执行，通常在单独的 goroutine 中调用。
func (s *DiscoverInstance) Watcher() {
	// 轮询间隔使用 TTL 的一半：既避免过于频繁访问 API Server，又尽量及时感知变更。
	interval := time.Duration(s.conf.TTL) * time.Second / 2
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			// Unwatch() 调用 cancel 后，退出 Watcher。
			return
		case <-ticker.C:
			// 定时轮询：拉取最新 ConfigMap 快照，若 resourceVersion 变化则重建索引。
			if err := s.pullAndRebuildIfChanged(); err != nil {
				// 拉取失败时做小退避，避免异常时空转。
				timer := time.NewTimer(200 * time.Millisecond)
				select {
				case <-s.ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}
	}
}

// Unwatch 停止监听并释放相关资源。
func (s *DiscoverInstance) Unwatch() {
	s.cancel()
}

// WithLog 设置内部日志输出回调。
func (s *DiscoverInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	s.log = handle
}

func (s *DiscoverInstance) bootstrap() error {
	// 首次快照：直接拉取 ConfigMap 并构建本地索引。
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	cm, err := s.client.CoreV1().ConfigMaps(s.conf.Namespace).Get(ctx, configMapName(s.meta.Env), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			s.mu.Lock()
			s.service = make(micro.ServiceDiscover)
			s.method = make(micro.ServiceMethod)
			s.resourceVersion = ""
			s.mu.Unlock()
			return nil
		}
		return err
	}

	// 记录 resourceVersion，Watcher 后续用它判断是否有变化。
	s.rebuild(cm, cm.ResourceVersion)

	if s.log != nil {
		s.log(logger.Info, fmt.Sprintf("Bootstrap completed, discovered %d services", len(s.service)))
	}

	return nil
}

func (s *DiscoverInstance) pullAndRebuildIfChanged() error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	cm, err := s.client.CoreV1().ConfigMaps(s.conf.Namespace).Get(ctx, configMapName(s.meta.Env), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			s.mu.Lock()
			s.service = make(micro.ServiceDiscover)
			s.method = make(micro.ServiceMethod)
			s.resourceVersion = ""
			s.mu.Unlock()
			return nil
		}
		return err
	}

	s.mu.RLock()
	currentRV := s.resourceVersion
	s.mu.RUnlock()

	// 未变化则跳过重建，降低 CPU 与锁竞争。
	if cm.ResourceVersion != "" && cm.ResourceVersion == currentRV {
		return nil
	}

	s.rebuild(cm, cm.ResourceVersion)

	return nil
}

func (s *DiscoverInstance) rebuild(cm *corev1.ConfigMap, resourceVersion string) {
	// 用新的 map 先构建完整快照，再一次性替换旧索引，避免“构建一半”被 GetService 读到。
	nextService := make(micro.ServiceDiscover)
	nextMethod := make(micro.ServiceMethod)

	// ConfigMap.data 的每个 value 都应是 ServiceNode(JSON)。
	now := time.Now()
	staleAfter := time.Duration(s.conf.TTL) * time.Second
	for _, raw := range cm.Data {
		var node micro.ServiceNode
		if err := json.Unmarshal([]byte(raw), &node); err != nil {
			if s.log != nil {
				s.log(logger.Error, fmt.Sprintf("Failed to unmarshal service node: %s", err.Error()))
			}
			continue
		}

		// 过滤不完整/不属于当前环境的数据，避免缓存被污染。
		if node.Meta == nil || node.Meta.AppId == "" || node.Meta.Env == "" || node.RunDate == "" {
			continue
		}
		if node.Meta.Env != s.meta.Env {
			continue
		}
		parsedRunDate, err := time.ParseInLocation(time.DateTime, node.RunDate, time.Local)
		if err != nil {
			continue
		}
		if staleAfter > 0 && now.Sub(parsedRunDate) > staleAfter {
			continue
		}

		nodes := nextService[node.Meta.AppId]
		copied := node
		nodes = append(nodes, &copied)
		nextService[node.Meta.AppId] = nodes
	}

	for appId := range nextService {
		nodes := nextService[appId]
		sort.Slice(nodes, func(i, j int) bool {
			li, err1 := time.ParseInLocation(time.DateTime, nodes[i].RunDate, time.Local)
			lj, err2 := time.ParseInLocation(time.DateTime, nodes[j].RunDate, time.Local)
			if err1 != nil && err2 != nil {
				return nodes[i].LeaseId > nodes[j].LeaseId
			}
			if err1 != nil {
				return false
			}
			if err2 != nil {
				return true
			}
			return li.After(lj)
		})
		nextService[appId] = nodes
	}

	// 派生索引：由 nextService 推导 nextMethod，保证两者一致。
	for appId, nodes := range nextService {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			for sm := range node.Methods {
				nextMethod[sm] = appId
			}
		}
	}

	s.mu.Lock()
	s.service = nextService
	s.method = nextMethod
	s.resourceVersion = resourceVersion
	s.mu.Unlock()
}
