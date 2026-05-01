package kernel

const (
	Language = "Golang"
	Version  = "v1.0.0"
)

type Config struct {
	// 内核开发语言
	Language string
	// 内核版本
	Version string
}

func (c *Config) Normalize() {
	c.Language = Language
	c.Version = Version
}
