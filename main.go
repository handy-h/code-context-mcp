package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Version information injected at build time
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// 加载 .env 配置文件
	// 按优先级尝试：1.当前目录 2.可执行文件所在目录
	godotenv.Load(".env")
	if exe, err := os.Executable(); err == nil {
		godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
	}

	// 命令行参数
	indexPath := flag.String("index", "", "索引模式：指定项目根目录路径，构建向量索引后退出")
	showVersion := flag.Bool("version", false, "显示版本信息")
	flag.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("code-context-mcp %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	cfg := LoadConfig()

	// 验证必要配置
	if cfg.ZillizURI == "" || cfg.ZillizToken == "" {
		fmt.Fprintln(os.Stderr, "错误: 请配置 ZILLIZ_URI 和 ZILLIZ_TOKEN 环境变量")
		fmt.Fprintln(os.Stderr, "可创建 .env 文件或设置系统环境变量")
		os.Exit(1)
	}

	// 命令行索引模式
	if *indexPath != "" {
		runIndexMode(*indexPath, cfg)
		return
	}

	// MCP 服务器模式 (stdio)
	runMCPMode(cfg)
}

func runIndexMode(projectPath string, cfg Config) {
	log.Printf("索引模式: 扫描项目 %s", projectPath)

	ctx := context.Background()
	vdb, err := NewVectorDB(ctx, cfg)
	if err != nil {
		log.Fatalf("连接向量数据库失败: %v", err)
	}
	defer vdb.Close()

	invIndex := NewInvertedIndex()
	stats, err := BuildIndex(ctx, projectPath, cfg, vdb, invIndex)
	if err != nil {
		log.Fatalf("索引构建失败: %v", err)
	}

	log.Printf("索引完成: %d 个文件, %d 个代码片段", stats.TotalFiles, stats.TotalChunks)
}

func runMCPMode(cfg Config) {
	var indexMgr *IndexManager
	if cfg.ProjectPath != "" && cfg.AutoIndex {
		indexMgr = NewIndexManager(cfg, cfg.ProjectPath)
		ctx := context.Background()
		// 后台异步构建索引，不阻塞 MCP 服务启动
		// 避免全量构建耗时超过客户端初始化超时（通常 50s）
		go func() {
			if err := indexMgr.CheckAndAutoIndex(ctx); err != nil {
				log.Printf("自动索引失败: %v（请稍后手动调用 index_project 重试）", err)
			}
		}()
	}

	server := NewMCPServer(cfg)
	RegisterTools(server, cfg, indexMgr)

	if err := server.Run(); err != nil {
		log.Fatalf("MCP 服务器错误: %v", err)
	}
}
