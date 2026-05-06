package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/handy-h/code-context-mcp/internal/config"
)

// ================= JSON-RPC 2.0 数据结构 =================

// ================= JSON-RPC 2.0 数据结构 =================

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ================= MCP 协议数据结构 =================

// type initializeParams struct {
// 	ProtocolVersion string                 `json:"protocolVersion"`
// 	Capabilities    map[string]interface{} `json:"capabilities"`
// 	ClientInfo      struct {
// 		Name    string `json:"name"`
// 		Version string `json:"version"`
// 	} `json:"clientInfo"`
// }

type initializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
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
	}
}

type toolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ================= MCP Server =================

// MCPServer MCP 服务器
type MCPServer struct {
	cfg   config.Config
	tools map[string]ToolHandler
}

// ToolHandler 工具处理函数
type ToolHandler func(args map[string]interface{}) (string, error)

// NewMCPServer 创建 MCP 服务器
func NewMCPServer(cfg config.Config) *MCPServer {
	return &MCPServer{
		cfg:   cfg,
		tools: make(map[string]ToolHandler),
	}
}

// RegisterTool 注册工具
func (s *MCPServer) RegisterTool(name string, handler ToolHandler) {
	s.tools[name] = handler
}

// Run 启动 MCP 服务器（stdio 模式）
func (s *MCPServer) Run() error {
	log.Println("MCP 服务器启动 (stdio 模式)")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("解析请求失败: %v", err)
			continue
		}

		resp := s.handleRequest(req)
		// 通知类消息不返回响应（JSON-RPC 规范）
		if resp.JSONRPC == "" {
			continue
		}

		respBytes, err := json.Marshal(resp)
		if err != nil {
			log.Printf("序列化响应失败: %v", err)
			continue
		}

		fmt.Println(string(respBytes))
	}

	return scanner.Err()
}

func (s *MCPServer) handleRequest(req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		// 客户端初始化完成通知，JSON-RPC 规范要求不返回响应
		return jsonRPCResponse{}
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32601, Message: fmt.Sprintf("方法不存在: %s", req.Method)},
		}
	}
}

func (s *MCPServer) handleInitialize(req jsonRPCRequest) jsonRPCResponse {
	result := initializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": false,
			},
		},
	}
	result.ServerInfo.Name = "code-context-mcp"
	result.ServerInfo.Version = "1.0.0"

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func (s *MCPServer) handleToolsList(req jsonRPCRequest) jsonRPCResponse {
	tools := GetToolDefinitions()
	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  toolsListResult{Tools: tools},
	}
}

func (s *MCPServer) handleToolsCall(req jsonRPCRequest) jsonRPCResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: "无效的参数"},
		}
	}

	handler, ok := s.tools[params.Name]
	if !ok {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolCallResult{
				Content: []toolContent{{Type: "text", Text: fmt.Sprintf("工具不存在: %s", params.Name)}},
				IsError: true,
			},
		}
	}

	resultText, err := handler(params.Arguments)
	if err != nil {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolCallResult{
				Content: []toolContent{{Type: "text", Text: fmt.Sprintf("工具执行错误: %v", err)}},
				IsError: true,
			},
		}
	}

	return jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolCallResult{
			Content: []toolContent{{Type: "text", Text: resultText}},
		},
	}
}
