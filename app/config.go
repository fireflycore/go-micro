package app

type Config struct {
	// 应用id
	Id string `json:"id"`
	// 应用环境
	Env string `json:"env"`
	// 应用名称
	Name string `json:"name"`
	// 应用密钥
	Secret string `json:"secret"`
	// 应用版本
	Version string `json:"version"`
	// 实例id
	InstanceId string `json:"instance_id"`
}
