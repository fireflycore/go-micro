// Package kubernetes 提供基于 Kubernetes Service 的服务注册与发现实现。
// 在 Istio/K8s 环境下，服务注册由 K8s 托管，服务发现由 K8s DNS/Envoy 托管。
// 本实现主要用于适配 go-micro 的接口，将 K8s Service 映射为 ServiceNode。
package kubernetes

import (
	"github.com/fireflycore/go-micro/logger"
	micro "github.com/fireflycore/go-micro/registry"
)

// NewRegister 创建基于 Kubernetes 的服务注册适配器。
// 在 K8s 模式下，这是一个 No-Op 实现，因为 Pod 生命周期由 K8s 管理。
func NewRegister(meta *micro.Meta, config *micro.ServiceConf) (*RegisterInstance, error) {
	return &RegisterInstance{
		meta:   meta,
		config: config,
	}, nil
}

type RegisterInstance struct {
	meta   *micro.Meta
	config *micro.ServiceConf
	log    func(level logger.LogLevel, message string)
}

// Install 不执行任何操作。
func (r *RegisterInstance) Install(service *micro.ServiceNode) error {
	if r.log != nil {
		r.log(logger.Info, "Kubernetes register: Install is ignored (managed by K8s)")
	}
	return nil
}

// Uninstall 不执行任何操作。
func (r *RegisterInstance) Uninstall() {
	if r.log != nil {
		r.log(logger.Info, "Kubernetes register: Uninstall is ignored (managed by K8s)")
	}
}

// SustainLease 不执行任何操作。
func (r *RegisterInstance) SustainLease() {}

// WithRetryBefore 不执行任何操作。
func (r *RegisterInstance) WithRetryBefore(handle func()) {}

// WithRetryAfter 不执行任何操作。
func (r *RegisterInstance) WithRetryAfter(handle func()) {}

// WithLog 设置日志回调。
func (r *RegisterInstance) WithLog(handle func(level logger.LogLevel, message string)) {
	r.log = handle
}
