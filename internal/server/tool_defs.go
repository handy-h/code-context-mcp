package server

// ToolDefinition MCP 工具定义
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// toolsListResult 工具列表响应
type toolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// GetToolDefinitions 返回所有工具的定义
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
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
		{
			Name:        "token_stats",
			Description: "查看 MCP 工具使用期间累计节省的 token 统计（按日维度）。基于基线对照法估算。",
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
	}
}
