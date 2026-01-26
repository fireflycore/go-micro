package registry

// ServiceNode 适用于服务注册/发现的节点描述。
type ServiceNode struct {
	ProtoCount int             `json:"proto_count"`
	LeaseId    int             `json:"lease_id"`
	Weight     int             `json:"weight"` // 权重，默认100
	RunDate    string          `json:"run_date"`
	Methods    map[string]bool `json:"methods"`

	Network *Network `json:"network"`
	Kernel  *Kernel  `json:"kernel"`
	Meta    *Meta    `json:"meta"`
}

// ParseMethod 将节点方法映射写入方法表（method -> appId）。
func (ist *ServiceNode) ParseMethod(s ServiceMethod) {
	if ist.Meta == nil || ist.Meta.AppId == "" {
		return
	}
	for k := range ist.Methods {
		s[k] = ist.Meta.AppId
	}
}

// CheckMethod 检查节点是否包含指定方法。
func (ist *ServiceNode) CheckMethod(sm string) error {
	if _, ok := ist.Methods[sm]; ok {
		return nil
	}
	return ErrServiceNodeMethodNotExists
}
