package utils

import (
	"net"
	"strings"
)

// GetInternalNetworkIp 获取内网Ip
func GetInternalNetworkIp() string {
	// 通过建立 UDP “伪连接”获取本机路由选路后的本地地址，不会真正发送业务数据。
	dial, err := net.Dial("udp", "114.114.114.114:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer dial.Close()
	addr := dial.LocalAddr().String()

	index := strings.LastIndex(addr, ":")
	return addr[:index]
}
