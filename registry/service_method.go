package registry

// ServiceMethod 服务方法映射（method -> appId）。
type ServiceMethod map[string]string

// GetAppId 根据 gRPC 方法名返回归属的 appId。
func (s ServiceMethod) GetAppId(sm string) (string, error) {
	if v, ok := s[sm]; ok {
		return v, nil
	}
	return "", ErrServiceMethodNotExists
}
