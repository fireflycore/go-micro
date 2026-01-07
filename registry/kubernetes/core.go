// Package kubernetes 提供基于 Kubernetes API 的服务注册与服务发现实现。
//
// 设计思路（对齐 etcd/consul 的能力边界）：
// - 用 ConfigMap 充当“注册中心存储”，data 中每条记录保存一个 ServiceNode(JSON)；
// - Register 负责写入/更新/删除对应 key；SustainLease 通过周期性刷新自身记录模拟租约心跳；
// - Discovery 负责读取 ConfigMap 并构建本地索引；Watcher 通过轮询刷新本地缓存。
//
// 约束说明：
// - 该实现使用 client-go 访问 Kubernetes API；
// - ServiceConf.Namespace 视为 Kubernetes namespace；
// - Meta.Env 用于区分不同环境，映射到 ConfigMap 名称（ff-registry-<env>）。
package kubernetes

// configMapName 生成存储注册信息的 ConfigMap 名称。
// 命名约定：ff-registry-<env>，确保不同环境数据隔离。
func configMapName(env string) string {
	if env == "" {
		env = "default"
	}
	return "ff-registry-" + env
}
