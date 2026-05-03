package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// getToolDefinitions 返回所有工具的定义
func getToolDefinitions() []toolDefinition {
	return []toolDefinition{
		{
			Name:        "code_search",
			Description: "语义搜索代码片段。通过自然语言描述搜索项目中相关的代码，返回最匹配的代码片段及其文件路径和相似度。比关键词搜索更精准，能理解代码语义。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "搜索查询，用自然语言描述你想找的代码，如：'处理用户认证的逻辑' 或 'OCR识别相关代码'",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"description": "返回结果数量，默认5",
						"default":     5,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "file_context",
			Description: "获取指定文件的完整内容或结构摘要。当需要查看某个文件的完整代码时使用。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "文件路径（相对于项目根目录）",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"full", "summary"},
						"default":     "full",
						"description": "返回模式：full(完整代码)/summary(结构摘要)",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "index_project",
			Description: "将项目代码索引到向量数据库。首次使用或代码有较大变更后需要运行，会扫描项目文件、切分、向量化并插入向量数据库。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "项目根目录的绝对路径",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "symbol_search",
			Description: "精确符号搜索。通过符号名查找定义和引用，返回按文件分组的结果。比语义搜索更适合精确符号查找，如查找字段的所有引用。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "符号名或关键词",
					},
					"search_type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"definition", "reference", "all"},
						"default":     "all",
						"description": "搜索类型：definition(仅定义)/reference(仅引用)/all(全部)",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"default":     20,
						"description": "返回结果数量，默认20",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "impact_analysis",
			Description: "影响范围分析。分析符号被删除/重命名/修改签名后，受影响的文件和代码位置，一次调用完成影响范围分析。",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"symbol": map[string]interface{}{
						"type":        "string",
						"description": "要分析的符号名",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"delete", "rename", "modify"},
						"description": "操作类型：delete(删除)/rename(重命名)/modify(修改签名)",
					},
					"new_name": map[string]interface{}{
						"type":        "string",
						"description": "重命名时的新名称（action=rename时必填）",
					},
				},
				"required": []string{"symbol", "action"},
			},
		},
	}
}

// RegisterTools 注册所有工具处理器
func RegisterTools(server *MCPServer, cfg Config, indexMgr *IndexManager) {
	// code_search: 语义搜索代码
	server.RegisterTool("code_search", func(args map[string]interface{}) (string, error) {
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

		// 过期检测，触发后台增量更新
		if indexMgr != nil {
			indexMgr.TriggerUpdateIfStale(context.Background())
		}

		// 1. 获取查询向量
		vector, err := GetEmbedding(cfg, query)
		if err != nil {
			return "", fmt.Errorf("向量化查询失败: %v", err)
		}

		// 2. 向量搜索
		ctx := context.Background()
		vdb, err := NewVectorDB(ctx, cfg)
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
	server.RegisterTool("file_context", func(args map[string]interface{}) (string, error) {
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
			lang := detectLanguage(filePath)
			summary := ExtractSummary(string(content), lang, filePath)
			data, err := json.MarshalIndent(summary, "", "  ")
			if err != nil {
				return "", fmt.Errorf("格式化摘要失败: %v", err)
			}
			return string(data), nil
		}

		return string(content), nil
	})

	// index_project: 索引项目代码
	server.RegisterTool("index_project", func(args map[string]interface{}) (string, error) {
		projectPath, _ := args["path"].(string)
		if projectPath == "" {
			return "", fmt.Errorf("path 参数不能为空")
		}

		log.Printf("index_project: path=%q", projectPath)

		ctx := context.Background()
		vdb, err := NewVectorDB(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("连接向量数据库失败: %v", err)
		}
		defer vdb.Close()

		var invIndex *InvertedIndex
		if indexMgr != nil {
			invIndex = indexMgr.GetInvertedIndex()
		} else {
			invIndex = NewInvertedIndex()
		}

		stats, err := BuildIndex(ctx, projectPath, cfg, vdb, invIndex)
		if err != nil {
			return "", fmt.Errorf("索引构建失败: %v", err)
		}

		// 保存索引状态
		if indexMgr != nil {
			stateStore := NewIndexStateStore(projectPath, cfg.IndexStatePath)
			currentFingerprint, currentMtimes, _ := stateStore.GetCurrentFingerprint(projectPath, cfg.ScanExtensions)
			state := &IndexState{
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
	server.RegisterTool("symbol_search", func(args map[string]interface{}) (string, error) {
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

	// impact_analysis: 影响范围分析
	server.RegisterTool("impact_analysis", func(args map[string]interface{}) (string, error) {
		symbol, _ := args["symbol"].(string)
		if symbol == "" {
			return "", fmt.Errorf("symbol 参数不能为空")
		}

		action, _ := args["action"].(string)
		if action == "" {
			return "", fmt.Errorf("action 参数不能为空")
		}

		newName, _ := args["new_name"].(string)

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
		result := ImpactResult{
			Symbol: symbol,
			Action: action,
			Summary: ImpactSummary{
				Categories: make(map[string]int),
			},
		}

		affectedFiles := make(map[string]bool)
		for _, occ := range occurrences {
			suggestion := generateSuggestion(action, symbol, newName, occ.Type)
			impactType := categorizeImpact(occ.Type, occ.Context)

			result.Impacts = append(result.Impacts, ImpactItem{
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
