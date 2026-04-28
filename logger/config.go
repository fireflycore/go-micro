package logger

type Config struct {
	EnableConsole bool `json:"console"`
	EnableRemote  bool `json:"remote"`
}
