package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/handy-h/code-context-mcp/internal/indexer"
	"github.com/handy-h/code-context-mcp/internal/search"
	"github.com/handy-h/code-context-mcp/internal/server"
	"github.com/handy-h/code-context-mcp/internal/tools"
)

// Version information injected at build time
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// 初始化结构化日志，写入 stderr（stdout 专用于 MCP JSON-RPC 通信）
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// 加载 .env 配置文件
	// 按优先级尝试：1.当前目录 2.可执行文件所在目录
	_ = godotenv.Load(".env")
	if exe, err := os.Executable(); err == nil {
		_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
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

	cfg := config.LoadConfig()

	// 验证必要配置
	if cfg.VectorStore == config.VectorStoreZilliz && (cfg.ZillizURI == "" || cfg.ZillizToken == "") {
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

func runIndexMode(projectPath string, cfg config.Config) {
	slog.Info("索引模式", "path", projectPath)

	ctx := context.Background()
	vdb, err := search.NewVectorDB(ctx, cfg)
	if err != nil {
		slog.Error("连接向量数据库失败", "err", err)
		os.Exit(1)
	}
	defer vdb.Close()

	invIndex := search.NewInvertedIndex()
	stats, err := indexer.BuildIndex(ctx, projectPath, cfg, vdb, invIndex)
	if err != nil {
		slog.Error("索引构建失败", "err", err)
		os.Exit(1)
	}

	slog.Info("索引完成", "files", stats.TotalFiles, "chunks", stats.TotalChunks)
}

func runMCPMode(cfg config.Config) {
	var indexMgr *indexer.IndexManager
	if cfg.ProjectPath != "" && cfg.AutoIndex {
		indexMgr = indexer.NewIndexManager(cfg, cfg.ProjectPath)
		ctx := context.Background()
		// 后台异步构建索引，不阻塞 MCP 服务启动
		// 避免全量构建耗时超过客户端初始化超时（通常 50s）
		go func() {
			if err := indexMgr.CheckAndAutoIndex(ctx); err != nil {
				slog.Error("自动索引失败", "err", err)
			}
		}()
	}

	srv := server.NewMCPServer(cfg, version)
	tools.RegisterTools(srv, cfg, indexMgr)

	if err := srv.Run(); err != nil {
		slog.Error("MCP 服务器错误", "err", err)
		os.Exit(1)
	}
}
