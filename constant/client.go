package constant

const (
	ClientTypeUnspecified = iota
	// ClientTypeWebBrowser 网页端浏览器
	ClientTypeWebBrowser
	// ClientTypeMiniProgram 小程序
	ClientTypeMiniProgram
	// ClientTypeMobileApp 移动端应用
	ClientTypeMobileApp
	// ClientTypeDesktopApp 桌面端应用
	ClientTypeDesktopApp
	// ClientTypeServer 服务端
	ClientTypeServer
	// ClientTypeEmbedded 嵌入式
	ClientTypeEmbedded
)
