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

// Constants for K8s labels and annotations
const (
	LabelAppId        = "micro.app_id"
	AnnotationMethods = "micro.methods"
	AnnotationVersion = "micro.version"
)

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
	// 1. 尝试获取 K8s 配置 (In-Cluster 或 KubeConfig)
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig (local dev)
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
		config.Namespace = "default" // Default K8s namespace
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

	// Return a copy
	out := append([]*micro.ServiceNode(nil), nodes...)
	return out, nil
}

func (d *DiscoverInstance) Watcher() {
	// 启动 Watch 循环
	go func() {
		for {
			select {
			case <-d.ctx.Done():
				return
			default:
			}

			// Watch Services with label selector
			// 这里假设所有微服务都打上了 micro.app_id 标签
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

func (d *DiscoverInstance) Unwatch() {
	d.cancel()
}

func (d *DiscoverInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	d.log = handle
}

func (d *DiscoverInstance) bootstrap() error {
	// List all services
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
	// K8s Service DNS: <svc>.<ns>.svc.cluster.local
	// Port: 优先取名为 "grpc" 的端口，否则取第一个端口
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

	// 如果使用了 Istio，通常 ClusterIP 就足够了，Envoy 会拦截
	// 我们也可以直接用 IP，但 DNS 更稳健

	node := &micro.ServiceNode{
		ProtoCount: 1, // 假定值
		LeaseId:    0, // K8s 无需 lease
		RunDate:    svc.CreationTimestamp.Format(time.DateTime),
		Methods:    methodMap,
		Network: &micro.Network{
			Internal: host, // 关键：指向 K8s Service DNS
			External: host, // 外部通常通过 Ingress，这里简化为相同
		},
		Kernel: &micro.Kernel{
			Language: "Golang", // 默认
		},
		Meta: &micro.Meta{
			AppId:   appId,
			Env:     d.meta.Env, // 假设同环境
			Version: svc.Annotations[AnnotationVersion],
		},
	}

	// 更新缓存
	// 注意：在 K8s 中，一个 Service 通常对应一组 Pod，但对外只有一个 Service IP/DNS。
	// 所以我们这里只维护一个 Node 即可。
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
