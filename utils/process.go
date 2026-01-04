package utils

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// WatcherProcess 监听系统信号，阻塞进程直到接收到终止信号
// handle: 接收到信号后的回调函数
func WatcherProcess(handle func()) {
	fmt.Println("----- watch process start -----")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	// 接收信号
	_ = <-ch

	if handle != nil {
		handle()
	}

	fmt.Println("----- watch process end -----")
}
