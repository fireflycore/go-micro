package registry

// Discovery 定义服务发现实现的最小能力集合。
type Discovery interface {
	// GetService 根据方法名返回可用节点列表。
	GetService(name string) ([]*ServiceNode, error)
	// Watcher 启动监听并持续刷新本地缓存。
	Watcher()
	// Unwatch 停止监听并释放相关资源。
	Unwatch()
}

// ServiceDiscover 服务发现数据结构（appId -> nodes）。
type ServiceDiscover map[string][]*ServiceNode

// GetNodes 获取指定 appId 下的所有服务节点。
func (s ServiceDiscover) GetNodes(appId string) ([]*ServiceNode, error) {
	if v, ok := s[appId]; ok {
		return v, nil
	}
	return nil, ErrServiceNodeNotExists
}

// ServiceMethods 服务方法映射（method -> appId）。
type ServiceMethods map[string]string

// GetAppId 根据 gRPC 方法名返回归属的 appId。
func (s ServiceMethods) GetAppId(sm string) (string, error) {
	if v, ok := s[sm]; ok {
		return v, nil
	}
	return "", ErrServiceMethodNotExists
}
