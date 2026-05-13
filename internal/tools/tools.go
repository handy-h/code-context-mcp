package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/handy-h/code-context-mcp/internal/embedding"
	"github.com/handy-h/code-context-mcp/internal/indexer"
	"github.com/handy-h/code-context-mcp/internal/search"
	"github.com/handy-h/code-context-mcp/internal/server"
	"github.com/handy-h/code-context-mcp/internal/types"
	"github.com/handy-h/code-context-mcp/pkg/file"
	"github.com/handy-h/code-context-mcp/pkg/structure"
)

// RegisterTools 注册所有工具处理器
func RegisterTools(srv *server.MCPServer, cfg config.Config, indexMgr *indexer.IndexManager) {
	// code_search: 语义搜索代码
	srv.RegisterTool("code_search", func(args map[string]interface{}) (string, error) {
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query 参数不能为空")
		}

		topK := 5
		if tk, ok := args["top_k"]; ok {
			switch v := tk.(type) {
			case float64:
				topK = int(v)
			case json.Number:
				if n, err := v.Int64(); err == nil {
					topK = int(n)
				}
			case string:
				if n, err := strconv.Atoi(v); err == nil {
					topK = n
				}
			}
		}

		log.Printf("code_search: query=%q, top_k=%d", query, topK)

		// 创建超时context
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 过期检测，触发后台增量更新
		if indexMgr != nil {
			indexMgr.TriggerUpdateIfStale(ctx)
		}

		// 1. 获取查询向量
		vector, err := embedding.GetEmbedding(cfg, query)
		if err != nil {
			return "", fmt.Errorf("向量化查询失败: %v", err)
		}

		// 2. 向量搜索
		vdb, err := search.NewVectorDB(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("连接向量数据库失败: %v", err)
		}
		defer vdb.Close()

		results, err := vdb.Search(ctx, vector, topK)
		if err != nil {
			return "", fmt.Errorf("搜索失败: %v", err)
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
	})

	// file_context: 获取文件完整内容或结构摘要
	srv.RegisterTool("file_context", func(args map[string]interface{}) (string, error) {
		filePath, _ := args["file_path"].(string)
		if filePath == "" {
			return "", fmt.Errorf("file_path 参数不能为空")
		}

		mode := "full"
		if m, ok := args["mode"].(string); ok && m != "" {
			mode = m
		}

		log.Printf("file_context: path=%q, mode=%q", filePath, mode)

		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("读取文件失败: %v", err)
		}

		if mode == "summary" {
			lang := structure.DetectLanguage(filePath)
			summary := file.ExtractSummary(string(content), lang, filePath)
			data, err := json.MarshalIndent(summary, "", "  ")
			if err != nil {
				return "", fmt.Errorf("格式化摘要失败: %v", err)
			}
			return string(data), nil
		}

		return string(content), nil
	})

	// index_project: 索引项目
	srv.RegisterTool("index_project", func(args map[string]interface{}) (string, error) {
		projectPath, _ := args["path"].(string)
		if projectPath == "" {
			return "", fmt.Errorf("path 参数不能为空")
		}

		log.Printf("index_project: path=%q", projectPath)

		// 创建超时context（5分钟）
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		vdb, err := search.NewVectorDB(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("连接向量数据库失败: %v", err)
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
			return "", fmt.Errorf("索引构建失败: %v", err)
		}

		// 保存索引状态
		if indexMgr != nil {
			stateStore := indexer.NewIndexStateStore(projectPath, cfg.IndexStatePath)
			currentFingerprint, currentMtimes, _ := stateStore.GetCurrentFingerprint(projectPath, cfg.ScanExtensions)
			state := &types.IndexState{
				LastIndexedAt: time.Now(),
				Fingerprint:   currentFingerprint,
				TotalFiles:    stats.TotalFiles,
				TotalChunks:   stats.TotalChunks,
				ProjectPath:   projectPath,
				FileMtimes:    currentMtimes,
			}
			if saveErr := stateStore.Save(state); saveErr != nil {
				log.Printf("保存索引状态失败: %v", saveErr)
			}
		}

		return fmt.Sprintf("索引构建完成！共扫描 %d 个文件，生成 %d 个代码片段。", stats.TotalFiles, stats.TotalChunks), nil
	})

	// symbol_search: 精确符号搜索
	srv.RegisterTool("symbol_search", func(args map[string]interface{}) (string, error) {
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("query 参数不能为空")
		}

		searchType := "all"
		if st, ok := args["search_type"].(string); ok && st != "" {
			searchType = st
		}

		topK := 20
		if tk, ok := args["top_k"]; ok {
			switch v := tk.(type) {
			case float64:
				topK = int(v)
			case json.Number:
				if n, err := v.Int64(); err == nil {
					topK = int(n)
				}
			case string:
				if n, err := strconv.Atoi(v); err == nil {
					topK = n
				}
			}
		}

		log.Printf("symbol_search: query=%q, search_type=%q, top_k=%d", query, searchType, topK)

		// 创建超时context
		_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

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
	})

	// impact_analysis: 影响分析
	srv.RegisterTool("impact_analysis", func(args map[string]interface{}) (string, error) {
		symbol, _ := args["symbol"].(string)
		if symbol == "" {
			return "", fmt.Errorf("symbol 参数不能为空")
		}

		action, _ := args["action"].(string)
		if action == "" {
			return "", fmt.Errorf("action 参数不能为空")
		}

		newName, _ := args["new_name"].(string)

		// 创建超时context
		_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Printf("impact_analysis: symbol=%q, action=%q, new_name=%q", symbol, action, newName)

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
			return "", fmt.Errorf("格式化影响分析结果失败: %v", err)
		}
		return string(data), nil
	})
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
