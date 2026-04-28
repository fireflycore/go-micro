package example

import (
	"github.com/fireflycore/go-micro/app"
	"github.com/fireflycore/go-micro/kernel"
	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/service"
	"github.com/fireflycore/go-micro/telemetry"
)

type BootstrapConfig struct {
	// App 应用配置
	App app.Config `json:"app"`
	// Kernel 内核配置
	Kernel kernel.Config `json:"kernel"`
	// Logger 日志配置
	Logger logger.Config `json:"logger"`
	// Service 服务配置
	Service service.Config `json:"service"`
	// Telemetry 可观测性配置
	Telemetry telemetry.Config `json:"telemetry"`

	// ServerPort 服务端口
	ServerPort uint `json:"server_port"`
	// ManagePort 管理端口
	ManagedPort uint `json:"managed_port"`
	// LoadConfigMode 加载配置方式
	LoadConfigMode string `json:"load_config_mode"`
}
