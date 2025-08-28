/*
 * @Description:
 * @Author: 安知鱼
 * @Date: 2025-06-28 00:21:55
 * @LastEditTime: 2025-08-28 18:39:14
 * @LastEditors: 安知鱼
 */
package main

import (
	"embed"
	"log"
	"os"

	"github.com/anzhiyu-c/anheyu-app/cmd/server"
)

//go:embed assets/dist
var content embed.FS

func main() {
	// 调用位于 cmd/server 包中的 NewApp 函数来构建整个应用
	app, cleanup, err := server.NewApp(content)
	if err != nil {
		log.Fatalf("应用初始化失败: %v", err)
	}

	// 使用 defer 来确保 cleanup 函数在 main 退出时被调用
	defer cleanup()

	// 确保后台任务在程序退出时被停止
	defer app.Stop()

	if os.Getenv("ANHEYU_LICENSE_KEY") == "" {
		app.PrintBanner()
	}

	// 启动应用
	if err := app.Run(); err != nil {
		log.Fatalf("应用运行失败: %v", err)
	}
}
