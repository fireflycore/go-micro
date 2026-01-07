package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RegisterInstance 基于 Kubernetes ConfigMap 的服务注册实例。
//
// 语义对齐：
// - etcd 的 “key + lease” 模型 => ConfigMap.data 的 “key + 心跳更新时间” 模型；
// - SustainLease 周期性刷新自身记录，模拟租约保活；
// - Uninstall 删除自身 key，模拟租约撤销。
type RegisterInstance struct {
	// ctx/cancel 控制注册实例生命周期：
	// - SustainLease 会阻塞运行，收到 ctx.Done() 后退出
	// - Uninstall() 调用 cancel() 并删除本实例注册信息
	ctx    context.Context
	cancel context.CancelFunc

	// client 是访问 Kubernetes API 的 client-go 客户端。
	client kubernetes.Interface

	// meta/conf 来自上层注入；Install 会把它们写入 ServiceNode。
	meta *micro.Meta
	conf *micro.ServiceConf

	// retryCount 记录已重试次数（用于心跳失败时退避重试）。
	retryCount uint32
	// retryBefore 在每次重试前回调（用于指标/告警等）。
	retryBefore func()
	// retryAfter 在重试成功后回调（用于恢复通知等）。
	retryAfter func()

	// log 用于输出实现内部状态（对齐 etcd/consul 的模式）。
	log func(level logger.LogLevel, message string)

	// leaseId 用作该实例的“稳定唯一标识”，用于定位自身在 ConfigMap.data 的 key。
	// 注意：Kubernetes 没有与 etcd LeaseID 完全等价的概念，这里以本地生成值对齐语义。
	leaseId int

	// lastNode 缓存最后一次注册的服务节点信息：
	// - 心跳/重试时用于重新序列化写入；
	// - 便于在重试中更新 RunDate。
	lastNode *micro.ServiceNode
}

// NewRegister 创建基于 Kubernetes 的服务注册实例。
//
// 约定：
// - conf.Namespace 视为 Kubernetes namespace；
// - ConfigMap 名称固定为 ff-registry-<env>（env 来自 meta.Env）。
func NewRegister(client kubernetes.Interface, meta *micro.Meta, conf *micro.ServiceConf) (micro.Register, error) {
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

	// ctx 用于控制 SustainLease 的退出；Uninstall 会触发 cancel。
	ctx, cancel := context.WithCancel(context.Background())

	// leaseId 使用纳秒时间戳生成，尽量降低冲突概率。
	leaseId := int(time.Now().UnixNano())
	if leaseId == 0 {
		leaseId = 1
	}

	ins := &RegisterInstance{
		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		meta:    meta,
		conf:    conf,
		leaseId: leaseId,
	}

	return ins, nil
}

// Install 补齐 ServiceNode 元信息并写入 ConfigMap.data。
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

	// 对齐 etcd/consul：Install 负责补齐节点的运行时信息。
	service.Meta = s.meta
	service.Kernel = s.conf.Kernel
	service.Network = s.conf.Network
	service.LeaseId = s.leaseId
	service.RunDate = time.Now().Format(time.DateTime)

	// 缓存节点信息，后续心跳/重试会复用它重新写入。
	s.lastNode = service

	// 首次写入：把节点序列化并写入 ConfigMap.data[key]。
	return s.register()
}

// Uninstall 删除本实例在 ConfigMap.data 中注册的 key，并停止心跳。
func (s *RegisterInstance) Uninstall() {
	// 先让 SustainLease 退出，避免并发写入/删除互相覆盖。
	s.cancel()

	// 没有完成 Install 前，lastNode 可能为空；此时无需删除。
	if s.meta == nil || s.meta.AppId == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// best-effort：删除失败不阻塞进程退出。
	_ = s.deleteDataKey(ctx, s.dataKey())
}

// SustainLease 通过周期性刷新自身记录模拟“租约心跳”。
func (s *RegisterInstance) SustainLease() {
	// 用 TTL/2 作为心跳间隔，降低过期风险；最小 1s，避免过频调用 API Server。
	interval := time.Duration(s.conf.TTL) * time.Second / 2
	if interval < time.Second {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			// Uninstall() 会触发 ctx.Done()，这里优雅退出。
			return
		case <-ticker.C:
			// 每次心跳都更新 RunDate，以便发现端可用作“最后活跃时间”。
			if s.lastNode != nil {
				s.lastNode.RunDate = time.Now().Format(time.DateTime)
			}

			// 心跳本质上就是一次 register()（覆盖写入自身 key）。
			if err := s.register(); err != nil {
				// 写入失败则进入退避重试；重试失败时直接退出（对齐 etcd/consul 的策略）。
				if !s.retryRegister() {
					return
				}
			} else if s.retryCount != 0 {
				// 一旦成功写入，清零重试计数，避免误判为一直失败。
				s.retryCount = 0
			}
		}
	}
}

// WithRetryBefore 设置重试前回调。
func (s *RegisterInstance) WithRetryBefore(handle func()) {
	s.retryBefore = handle
}

// WithRetryAfter 设置重试成功后回调。
func (s *RegisterInstance) WithRetryAfter(handle func()) {
	s.retryAfter = handle
}

// WithLog 设置内部日志输出回调。
func (s *RegisterInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	s.log = handle
}

func (s *RegisterInstance) dataKey() string {
	// key 结构对齐 etcd：{appId}/{leaseId}
	return fmt.Sprintf("%s/%d", s.meta.AppId, s.leaseId)
}

func (s *RegisterInstance) register() error {
	// 未 Install 前没有节点信息；此时不写入，保持幂等。
	if s.lastNode == nil {
		return nil
	}

	// value 用 JSON 序列化后的 ServiceNode；发现端会反序列化并刷新本地缓存。
	b, err := json.Marshal(s.lastNode)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	return s.upsertDataKey(ctx, s.dataKey(), string(b))
}

func (s *RegisterInstance) retryRegister() bool {
	// 重试策略（对齐 etcd/consul）：
	// - 每轮间隔 TTL*5 秒；
	// - 达到 MaxRetry 仍失败则放弃（SustainLease 退出）；
	// - 成功写入后清零计数并触发 retryAfter。
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
			s.log(logger.Info, fmt.Sprintf("kubernetes retry register: %d/%d", s.retryCount, s.conf.MaxRetry))
		}

		// 如果 ConfigMap 被意外删除，先确保它存在再尝试写入。
		if err := s.ensureConfigMap(); err != nil {
			continue
		}

		if err := s.register(); err != nil {
			if s.log != nil {
				s.log(logger.Error, fmt.Sprintf("kubernetes re-register failed: %v", err))
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

func (s *RegisterInstance) ensureConfigMap() error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	name := configMapName(s.meta.Env)
	ns := s.conf.Namespace

	_, err := s.client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string]string{},
	}

	_, err = s.client.CoreV1().ConfigMaps(ns).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (s *RegisterInstance) upsertDataKey(ctx context.Context, key, value string) error {
	if err := s.ensureConfigMap(); err != nil {
		return err
	}

	name := configMapName(s.meta.Env)
	ns := s.conf.Namespace

	var lastErr error
	for i := 0; i < 5; i++ {
		cm, err := s.client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				if ensureErr := s.ensureConfigMap(); ensureErr != nil {
					lastErr = ensureErr
					continue
				}
				lastErr = err
				continue
			}
			return err
		}

		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[key] = value

		_, err = s.client.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}
		if apierrors.IsConflict(err) {
			lastErr = err
			continue
		}
		return err
	}
	return lastErr
}

func (s *RegisterInstance) deleteDataKey(ctx context.Context, key string) error {
	name := configMapName(s.meta.Env)
	ns := s.conf.Namespace

	var lastErr error
	for i := 0; i < 5; i++ {
		cm, err := s.client.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if cm.Data == nil {
			return nil
		}
		if _, ok := cm.Data[key]; !ok {
			return nil
		}

		delete(cm.Data, key)

		_, err = s.client.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}
		if apierrors.IsConflict(err) {
			lastErr = err
			continue
		}
		return err
	}
	return lastErr
}
