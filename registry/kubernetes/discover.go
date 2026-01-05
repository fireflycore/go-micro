// Package kubernetes 提供基于 Kubernetes Service 的服务注册与发现实现。
package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Label/Annotation 常量约定：用于把 K8s Service 映射为 go-micro ServiceNode。
const (
	// LabelAppId 用于标识 service 归属的 appId（Label：key=appId）。
	LabelAppId = "micro.app_id"
	// AnnotationMethods 用于声明该 service 对外暴露的 gRPC 方法集合（逗号分隔）。
	AnnotationMethods = "micro.methods"
	// AnnotationVersion 用于声明该 service 的版本信息。
	AnnotationVersion = "micro.version"
)

// DiscoverInstance 服务发现实例
type DiscoverInstance struct {
	meta   *micro.Meta
	config *micro.ServiceConf
	client *kubernetes.Clientset

	ctx    context.Context
	cancel context.CancelFunc

	log func(level logger.LogLevel, message string)

	method  micro.ServiceMethod
	service micro.ServiceDiscover

	mu sync.RWMutex
}

// NewDiscover 创建基于 Kubernetes 的服务发现实例。
// 它会监控当前 Namespace 下带有 micro.app_id 标签的 Service。
func NewDiscover(meta *micro.Meta, config *micro.ServiceConf) (micro.Discovery, error) {
	// 1. 获取 K8s 配置（优先 In-Cluster，其次使用本地 kubeconfig）。
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		// 回退到本地 kubeconfig（便于本地开发调试）。
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf(ErrConfigFailedFormat, err)
		}
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf(ErrClientFailedFormat, err)
	}

	if config == nil {
		return nil, micro.ErrServiceConfigIsNil
	}
	if config.Namespace == "" {
		config.Namespace = "default"
	}

	ctx, cancel := context.WithCancel(context.Background())

	d := &DiscoverInstance{
		meta:    meta,
		config:  config,
		client:  clientset,
		ctx:     ctx,
		cancel:  cancel,
		method:  make(micro.ServiceMethod),
		service: make(micro.ServiceDiscover),
	}

	// 初始化加载
	if err = d.bootstrap(); err != nil {
		cancel()
		return nil, err
	}

	return d, nil
}

// GetService 根据 gRPC 方法名返回可用节点列表。
func (d *DiscoverInstance) GetService(sm string) ([]*micro.ServiceNode, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	appId, ok := d.method[sm]
	if !ok {
		return nil, micro.ErrServiceMethodNotExists
	}

	nodes, ok := d.service[appId]
	if !ok {
		return nil, micro.ErrServiceNodeNotExists
	}

	// 返回副本，避免调用方修改内部缓存。
	out := append([]*micro.ServiceNode(nil), nodes...)
	return out, nil
}

// Watcher 启动后台监听并持续刷新本地缓存。
func (d *DiscoverInstance) Watcher() {
	go func() {
		for {
			select {
			case <-d.ctx.Done():
				return
			default:
			}

			// 监听带有 micro.app_id 标签的 Service（LabelSelector=key 表示 label 存在）。
			watcher, err := d.client.CoreV1().Services(d.config.Namespace).Watch(d.ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s", LabelAppId),
			})

			if err != nil {
				if d.log != nil {
					d.log(logger.Error, fmt.Sprintf("k8s watch failed: %v", err))
				}
				time.Sleep(time.Second * 5)
				continue
			}

			d.handleWatch(watcher)
			watcher.Stop()
		}
	}()
}

func (d *DiscoverInstance) handleWatch(watcher watch.Interface) {
	for event := range watcher.ResultChan() {
		svc, ok := event.Object.(*corev1.Service)
		if !ok {
			continue
		}

		d.mu.Lock()
		switch event.Type {
		case watch.Added, watch.Modified:
			d.updateServiceLocked(svc)
		case watch.Deleted:
			d.deleteServiceLocked(svc)
		}
		d.mu.Unlock()
	}
}

// Unwatch 停止监听并释放相关资源。
func (d *DiscoverInstance) Unwatch() {
	d.cancel()
}

// WithLog 设置日志回调。
func (d *DiscoverInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	d.log = handle
}

func (d *DiscoverInstance) bootstrap() error {
	services, err := d.client.CoreV1().Services(d.config.Namespace).List(d.ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s", LabelAppId),
	})
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, svc := range services.Items {
		d.updateServiceLocked(&svc)
	}
	return nil
}

func (d *DiscoverInstance) updateServiceLocked(svc *corev1.Service) {
	appId := svc.Labels[LabelAppId]
	if appId == "" {
		return
	}

	// 1. 解析 Methods
	methodsStr := svc.Annotations[AnnotationMethods]
	methodMap := make(map[string]bool)
	if methodsStr != "" {
		for _, m := range strings.Split(methodsStr, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				methodMap[m] = true
				d.method[m] = appId
			}
		}
	}

	// 2. 构建 ServiceNode
	// Service DNS：<svc>.<ns>.svc.cluster.local
	// Port：优先取名为 "grpc" 的端口，否则取第一个端口
	var port int32
	if len(svc.Spec.Ports) > 0 {
		port = svc.Spec.Ports[0].Port
		for _, p := range svc.Spec.Ports {
			if p.Name == "grpc" {
				port = p.Port
				break
			}
		}
	}

	host := fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, port)

	node := &micro.ServiceNode{
		ProtoCount: 1,
		LeaseId:    0,
		RunDate:    svc.CreationTimestamp.Format(time.DateTime),
		Methods:    methodMap,
		Network: &micro.Network{
			Internal: host,
			External: host,
		},
		Kernel: &micro.Kernel{
			Language: "Golang",
		},
		Meta: &micro.Meta{
			AppId:   appId,
			Env:     d.meta.Env,
			Version: svc.Annotations[AnnotationVersion],
		},
	}

	d.service[appId] = []*micro.ServiceNode{node}

	if d.log != nil {
		d.log(logger.Info, fmt.Sprintf("K8s Service updated: %s -> %s", appId, host))
	}
}

func (d *DiscoverInstance) deleteServiceLocked(svc *corev1.Service) {
	appId := svc.Labels[LabelAppId]
	if appId == "" {
		return
	}

	delete(d.service, appId)

	// 清理 method (需要遍历，效率较低但删除操作不频繁)
	for m, owner := range d.method {
		if owner == appId {
			delete(d.method, m)
		}
	}

	if d.log != nil {
		d.log(logger.Info, fmt.Sprintf("K8s Service removed: %s", appId))
	}
}
