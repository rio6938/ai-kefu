package main

import (
	"flag"
	"fmt"
	"os"

	"admin/server/pkg"
	"admin/server/pkg/config"
)

func main() {
	configPath := flag.String("c", config.DefaultPath, "配置文件路径")
	genConfig := flag.String("gen-config", "", "生成配置示例文件的输出路径")
	flag.Parse()

	// 生成示例文件不需要连数据库和 redis，必须在 pkg.Init 之前返回
	if *genConfig != "" {
		_, statErr := os.Stat(*genConfig)
		if err := config.WriteExample(*genConfig); err != nil {
			fmt.Fprintln(os.Stderr, "generate config example failed:", err)
			os.Exit(1)
		}
		if statErr == nil {
			fmt.Println("配置示例已覆盖:", *genConfig)
		} else {
			fmt.Println("配置示例已生成:", *genConfig)
		}
		return
	}

	if err := pkg.Init(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, "init failed:", err)
		os.Exit(1)
	}

	if err := pkg.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "server stopped:", err)
		os.Exit(1)
	}
}
