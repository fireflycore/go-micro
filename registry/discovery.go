package registry

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

type WatchEventFunc = func(event *ServiceEvent)

// Discovery 定义服务发现能力集合，主要面向网关路由场景。
type Discovery interface {
	// GetService 根据 RPC 方法名返回可用节点和所属 appId。
	// 例如方法 /user.UserService/Login 会映射到一个 appId 及其节点列表。
	GetService(method string) ([]*ServiceNode, string, error)

	// Watcher 启动发现监听并持续维护本地索引。
	// 该方法通常阻塞运行，内部负责同步 Method -> AppId 与 AppId -> Nodes 的映射关系。
	Watcher()
	// Unwatch 停止监听并释放内部资源。
	Unwatch()

	// WatchEvent 注册变更回调，供外部订阅服务增删改事件。
	// 回调触发时机由具体实现决定，调用方不应阻塞回调执行。
	WatchEvent(callback WatchEventFunc)
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
