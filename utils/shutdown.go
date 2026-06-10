package utils

import (
	"os"
	"os/signal"
	"syscall"
)

// ListenShutdown 监听关闭信号.
func ListenShutdown() {
	ch := make(chan os.Signal, 1)
	// 监听 SIGINT 和 SIGTERM 信号
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞等待信号
	<-ch
}
