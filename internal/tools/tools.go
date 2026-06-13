package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/handy-h/code-context-mcp/internal/embedding"
	"github.com/handy-h/code-context-mcp/internal/indexer"
	"github.com/handy-h/code-context-mcp/internal/search"
	"github.com/handy-h/code-context-mcp/internal/server"
	"github.com/handy-h/code-context-mcp/internal/types"
	"github.com/handy-h/code-context-mcp/pkg/file"
	"github.com/handy-h/code-context-mcp/pkg/structure"
)

// parseIntArg 从参数 map 中解析整数，兼容 float64/json.Number/string 三种来源
func parseIntArg(args map[string]interface{}, key string, defaultVal int) int {
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch tv := v.(type) {
	case float64:
		return int(tv)
	case json.Number:
		if n, err := tv.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(tv); err == nil {
			return n
		}
	}
	return defaultVal
}

// vdbFactory 共享向量数据库实例的工厂函数类型
type vdbFactory func(ctx context.Context) (search.VectorStore, error)

var (
	sharedVDB search.VectorStore
	vdbMu     sync.Mutex
	vdbInited bool
)

// getSharedVDB 获取或创建共享 VectorDB 实例，避免每次搜索请求新建连接
func getSharedVDB(cfg config.Config) vdbFactory {
	return func(ctx context.Context) (search.VectorStore, error) {
		vdbMu.Lock()
		defer vdbMu.Unlock()
		if !vdbInited {
			var err error
			sharedVDB, err = search.NewVectorDB(ctx, cfg)
			if err != nil {
				return nil, err
			}
			vdbInited = true
		}
		return sharedVDB, nil
	}
}

// RegisterTools 注册所有工具处理器
func RegisterTools(srv *server.MCPServer, cfg config.Config, indexMgr *indexer.IndexManager) {
	factory := getSharedVDB(cfg)
	srv.RegisterTool("code_search", handleCodeSearch(cfg, indexMgr, factory))
	srv.RegisterTool("file_context", handleFileContext(cfg))
	srv.RegisterTool("index_project", handleIndexProject(cfg, indexMgr))
	srv.RegisterTool("symbol_search", handleSymbolSearch(indexMgr))
	srv.RegisterTool("impact_analysis", handleImpactAnalysis(indexMgr))
}

// handleCodeSearch 语义搜索代码
func handleCodeSearch(cfg config.Config, indexMgr *indexer.IndexManager, getVDB vdbFactory) server.ToolHandler {
	return func(args map[string]interface{}) (string, error) {
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query 参数不能为空")
		}

		topK := parseIntArg(args, "top_k", 5)

		slog.Info("code_search", "query", query, "top_k", topK)

		ctx, cancel := context.WithTimeout(context.Background(), cfg.SearchTimeout)
		defer cancel()

		// 过期检测，触发后台增量更新
		if indexMgr != nil {
			indexMgr.TriggerUpdateIfStale(ctx)
		}

		// 1. 获取查询向量
		vector, err := embedding.GetEmbedding(cfg, query)
		if err != nil {
			return "", fmt.Errorf("向量化查询失败: %w", err)
		}

		// 2. 向量搜索（使用共享 VectorDB 实例）
		vdb, err := getVDB(ctx)
		if err != nil {
			return "", fmt.Errorf("连接向量数据库失败: %w", err)
		}

		results, err := vdb.Search(ctx, vector, topK)
		if err != nil {
			return "", fmt.Errorf("搜索失败: %w", err)
		}

		if len(results) == 0 {
			return "未找到相关代码。可能项目尚未索引，请先使用 index_project 工具。", nil
		}

		// 3. 格式化结果
		output := fmt.Sprintf("找到 %d 个相关代码片段：\n\n", len(results))
		for i, r := range results {
			output += fmt.Sprintf("--- 结果 %d (相似度: %.4f) ---\n", i+1, r.Score)
			output += fmt.Sprintf("文件: %s\n\n", r.File)
			// 截断过长的内容
			text := r.Text
			if len(text) > 1000 {
				text = text[:1000] + "\n... (已截断)"
			}
			output += text + "\n\n"
		}
		return output, nil
	}
}

// handleFileContext 获取文件完整内容或结构摘要
func handleFileContext(cfg config.Config) server.ToolHandler {
	return func(args map[string]interface{}) (string, error) {
		filePath, _ := args["file_path"].(string)
		if filePath == "" {
			return "", fmt.Errorf("file_path 参数不能为空")
		}

		mode := "full"
		if m, ok := args["mode"].(string); ok && m != "" {
			mode = m
		}

		// 防止目录遍历：基于项目根目录解析路径
		root := cfg.ProjectPath
		if root == "" {
			var err error
			root, err = os.Getwd()
			if err != nil {
				root = "."
			}
		}

		cleanPath := filepath.Clean(filePath)
		if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "..") {
			return "", fmt.Errorf("非法文件路径: 包含目录遍历")
		}

		fullPath := filepath.Join(root, cleanPath)
		fullPath = filepath.Clean(fullPath)

		// 解析符号链接，防止通过 symlink 绕过路径限制
		rootAbs, err1 := filepath.Abs(root)
		pathAbs, err2 := filepath.Abs(fullPath)
		if err1 == nil && err2 == nil {
			// 尝试解析符号链接获取真实路径
			realPath, evalErr := filepath.EvalSymlinks(pathAbs)
			if evalErr == nil {
				pathAbs = realPath
			}
			realRoot, rootEvalErr := filepath.EvalSymlinks(rootAbs)
			if rootEvalErr == nil {
				rootAbs = realRoot
			}

			sep := string(filepath.Separator)
			if !strings.HasPrefix(pathAbs, rootAbs+sep) && pathAbs != rootAbs {
				return "", fmt.Errorf("文件路径必须在项目目录内")
			}
		}

		slog.Info("file_context", "path", fullPath, "mode", mode)

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("读取文件失败: %w", err)
		}

		if mode == "summary" {
			lang := structure.DetectLanguage(filePath)
			summary := file.ExtractSummary(string(content), lang, filePath)
			data, err := json.MarshalIndent(summary, "", "  ")
			if err != nil {
				return "", fmt.Errorf("格式化摘要失败: %w", err)
			}
			return string(data), nil
		}

		return string(content), nil
	}
}

// handleIndexProject 索引项目
func handleIndexProject(cfg config.Config, indexMgr *indexer.IndexManager) server.ToolHandler {
	return func(args map[string]interface{}) (string, error) {
		projectPath, _ := args["path"].(string)
		if projectPath == "" {
			return "", fmt.Errorf("path 参数不能为空")
		}

		info, err := os.Stat(projectPath)
		if err != nil {
			return "", fmt.Errorf("项目路径不存在或无法访问: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("path 必须是目录")
		}

		slog.Info("index_project", "path", projectPath)

		ctx, cancel := context.WithTimeout(context.Background(), cfg.IndexTimeout)
		defer cancel()

		vdb, err := search.NewVectorDB(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("连接向量数据库失败: %w", err)
		}
		defer vdb.Close()

		var invIndex *search.InvertedIndex
		if indexMgr != nil {
			invIndex = indexMgr.GetInvertedIndex()
		} else {
			invIndex = search.NewInvertedIndex()
		}

		stats, err := indexer.BuildIndex(ctx, projectPath, cfg, vdb, invIndex)
		if err != nil {
			return "", fmt.Errorf("索引构建失败: %w", err)
		}

		// 保存索引状态
		if indexMgr != nil {
			stateStore := indexer.NewIndexStateStore(projectPath, cfg.IndexStatePath)
			if saveErr := stateStore.SaveFromStats(projectPath, stats, cfg.ScanExtensions); saveErr != nil {
				slog.Warn("保存索引状态失败", "err", saveErr)
			}
		}

		return fmt.Sprintf("索引构建完成！共扫描 %d 个文件，生成 %d 个代码片段。", stats.TotalFiles, stats.TotalChunks), nil
	}
}

// handleSymbolSearch 精确符号搜索
func handleSymbolSearch(indexMgr *indexer.IndexManager) server.ToolHandler {
	return func(args map[string]interface{}) (string, error) {
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query 参数不能为空")
		}

		searchType := "all"
		if st, ok := args["search_type"].(string); ok && st != "" {
			searchType = st
		}

		topK := parseIntArg(args, "top_k", 20)

		slog.Info("symbol_search", "query", query, "type", searchType, "top_k", topK)

		if indexMgr == nil {
			return "符号索引尚未构建，请先索引项目。", nil
		}

		invIndex := indexMgr.GetInvertedIndex()
		if invIndex == nil || invIndex.Size() == 0 {
			return "符号索引尚未构建，请先索引项目。", nil
		}

		results := invIndex.Search(query, searchType, topK)
		if len(results) == 0 {
			return fmt.Sprintf("未找到符号 %q 的匹配。", query), nil
		}

		// 按文件分组格式化输出
		output := fmt.Sprintf("找到 %d 个匹配位置：\n\n", len(results))
		currentFile := ""
		for _, r := range results {
			if r.File != currentFile {
				currentFile = r.File
				output += fmt.Sprintf("--- %s ---\n", r.File)
			}
			output += fmt.Sprintf("  L%d: [%s] %s\n", r.Line, r.Type, r.Context)
		}
		return output, nil
	}
}

// handleImpactAnalysis 影响范围分析
func handleImpactAnalysis(indexMgr *indexer.IndexManager) server.ToolHandler {
	return func(args map[string]interface{}) (string, error) {
		symbol, _ := args["symbol"].(string)
		if symbol == "" {
			return "", fmt.Errorf("symbol 参数不能为空")
		}

		action, _ := args["action"].(string)
		if action == "" {
			return "", fmt.Errorf("action 参数不能为空")
		}

		newName, _ := args["new_name"].(string)

		slog.Info("impact_analysis", "symbol", symbol, "action", action, "new_name", newName)

		if action == "rename" && newName == "" {
			return "", fmt.Errorf("action 为 rename 时必须提供 new_name 参数")
		}

		if indexMgr == nil {
			return "符号索引尚未构建，请先索引项目。", nil
		}

		invIndex := indexMgr.GetInvertedIndex()
		if invIndex == nil || invIndex.Size() == 0 {
			return "符号索引尚未构建，请先索引项目。", nil
		}

		occurrences := invIndex.GetAllOccurrences(symbol)
		if len(occurrences) == 0 {
			return fmt.Sprintf("未找到符号 %q 的任何出现位置。", symbol), nil
		}

		// 构建影响分析结果
		result := types.ImpactResult{
			Symbol: symbol,
			Action: action,
			Summary: types.ImpactSummary{
				Categories: make(map[string]int),
			},
		}

		affectedFiles := make(map[string]bool)
		for _, occ := range occurrences {
			suggestion := generateSuggestion(action, symbol, newName, occ.Type)
			impactType := categorizeImpact(occ.Type, occ.Context)

			result.Impacts = append(result.Impacts, types.ImpactItem{
				File:       occ.File,
				Line:       occ.Line,
				Type:       impactType,
				Context:    occ.Context,
				Suggestion: suggestion,
			})

			affectedFiles[occ.File] = true
			result.Summary.Categories[impactType]++
		}

		result.Summary.TotalFiles = len(affectedFiles)
		result.Summary.TotalReferences = len(result.Impacts)

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("格式化影响分析结果失败: %w", err)
		}
		return string(data), nil
	}
}

// generateSuggestion 根据操作类型生成修改建议
func generateSuggestion(action string, symbol string, newName string, occType string) string {
	typeLabel := "引用"
	if occType == "definition" {
		typeLabel = "定义"
	}

	switch action {
	case "delete":
		return fmt.Sprintf("删除该%s", typeLabel)
	case "rename":
		return fmt.Sprintf("将 %s 替换为 %s", symbol, newName)
	case "modify":
		if occType == "definition" {
			return "更新定义以匹配新签名"
		}
		return "更新调用以匹配新签名"
	default:
		return ""
	}
}

// categorizeImpact 对影响类型进行分类
func categorizeImpact(occType string, context string) string {
	if occType == "definition" {
		return "definition"
	}
	// 检查是否在 JSON 标签或 API payload 中
	if containsJSONTag(context) {
		return "api_payload"
	}
	return "reference"
}

func containsJSONTag(s string) bool {
	// 简单检查是否包含 json:" 标签
	for i := 0; i < len(s)-5; i++ {
		if s[i:i+6] == `json:"` {
			return true
		}
	}
	return false
}
