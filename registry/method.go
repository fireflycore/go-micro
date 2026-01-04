package registry

// ServiceMethods 服务方法映射（method -> appId）。
type ServiceMethods map[string]string

// GetAppId 根据 gRPC 方法名返回归属的 appId。
func (s ServiceMethods) GetAppId(sm string) (string, error) {
	if v, ok := s[sm]; ok {
		return v, nil
	}
	return "", ErrServiceMethodNotExists
}
