package kernel

const (
	Language = "Golang"
	Version  = "v1.0.0"
)

type Config struct {
	// 内核开发语言
	language string
	// 内核版本
	version string
}

func NewKernelConfig() *Config {
	return &Config{
		language: Language,
		version:  Version,
	}
}

func (c *Config) GetLanguage() string {
	return c.language
}

func (c *Config) GetVersion() string {
	return c.version
}
