package main

import (
	"legacy-video/internal/worker/combined"
)

// main 启动统一 worker 入口，同时注册转码与向量化 worker。
func main() {
	combined.Run()
}
