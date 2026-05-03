package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

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

type initializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type initializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type toolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ================= MCP Server =================

// MCPServer MCP 服务器
type MCPServer struct {
	cfg    Config
	tools  map[string]ToolHandler
}

// ToolHandler 工具处理函数
type ToolHandler func(args map[string]interface{}) (string, error)

// NewMCPServer 创建 MCP 服务器
func NewMCPServer(cfg Config) *MCPServer {
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
	tools := getToolDefinitions()
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
