package main

import "video-service/internal/worker/combined"

var runWorker = combined.Run

// main 启动统一 worker 入口，同时注册转码与向量化 worker。
func main() {
	runWorker()
}
