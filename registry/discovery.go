package registry

import "context"

// EventType 服务变动事件类型
type EventType int

const (
	EventAdd EventType = iota
	EventUpdate
	EventDelete
)

// ServiceEvent 服务变动事件
type ServiceEvent struct {
	Type    EventType
	Service *ServiceNode
}

// Discovery 定义服务发现实现的最小能力集合 (主要供网关使用)。
type Discovery interface {
	// GetService 根据 rpc method (如 /user.UserService/Login) 返回可用节点列表和对应的 AppId。
	// 网关在接收到请求时，只有 method 信息，需要通过此方法路由到具体的微服务实例。
	GetService(method string) ([]*ServiceNode, string, error)

	// Watch 启动全量监听，返回变动事件的通道，通过 ctx 控制生命周期。
	// 监听实现需要维护内部的 Method -> AppId 以及 AppId -> ServiceNodes 映射。
	Watch(ctx context.Context) (<-chan ServiceEvent, error)
	// Unwatch 停止监听并释放相关资源。
	Unwatch()
}

// ServiceDiscover 服务发现数据结构（appId -> nodes）本地缓存。
type ServiceDiscover map[string][]*ServiceNode

// GetNodes 获取指定 appId 下的所有服务节点。
func (s ServiceDiscover) GetNodes(appId string) ([]*ServiceNode, error) {
	if v, ok := s[appId]; ok {
		return v, nil
	}
	return nil, ErrServiceNodeNotExists
}
