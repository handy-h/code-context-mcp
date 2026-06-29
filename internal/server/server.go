package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/handy-h/code-context-mcp/internal/config"
	"github.com/handy-h/code-context-mcp/internal/tokenstats"
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

type initializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
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
	cfg     config.Config
	version string
	tools   map[string]ToolHandler
	tracker *tokenstats.Tracker // token 统计追踪器（可能为 nil）
	writeMu sync.Mutex          // 保护 stdout 写入的互斥锁
	wg      sync.WaitGroup
}

// ToolHandler 工具处理函数
type ToolHandler func(args map[string]interface{}) (string, error)

// NewMCPServer 创建 MCP 服务器
func NewMCPServer(cfg config.Config, version string) *MCPServer {
	return &MCPServer{
		cfg:     cfg,
		version: version,
		tools:   make(map[string]ToolHandler),
	}
}

// RegisterTool 注册工具
func (s *MCPServer) RegisterTool(name string, handler ToolHandler) {
	s.tools[name] = handler
}

// SetTracker 注入 token 统计追踪器
func (s *MCPServer) SetTracker(t *tokenstats.Tracker) {
	s.tracker = t
}

// Run 启动 MCP 服务器（stdio 模式）
// 支持并发处理：每个请求在独立 goroutine 中执行，避免长耗时工具阻塞其他请求
func (s *MCPServer) Run() error {
	slog.Info("MCP 服务器启动", "mode", "stdio")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			slog.Error("解析请求失败", "err", err)
			continue
		}

		// 通知类消息同步处理（无需响应）
		if req.ID == nil && (req.Method == "notifications/initialized") {
			s.handleRequest(req)
			continue
		}

		// 轻量级协议方法同步处理
		if isProtocolMethod(req.Method) {
			resp := s.handleRequest(req)
			if resp.JSONRPC == "" {
				continue
			}
			s.writeResponse(resp)
			continue
		}

		// 工具调用异步处理，避免阻塞后续请求
		s.wg.Add(1)
		go func(r jsonRPCRequest) {
			defer s.wg.Done()
			resp := s.handleRequest(r)
			if resp.JSONRPC == "" {
				return
			}
			s.writeResponse(resp)
		}(req)
	}

	// 等待所有进行中的请求完成
	s.wg.Wait()
	return scanner.Err()
}

// isProtocolMethod 判断是否为轻量级协议方法（同步处理）
func isProtocolMethod(method string) bool {
	switch method {
	case "initialize", "tools/list", "ping":
		return true
	}
	return false
}

// writeResponse 线程安全地写入 JSON-RPC 响应到 stdout
func (s *MCPServer) writeResponse(resp jsonRPCResponse) {
	respBytes, err := json.Marshal(resp)
	if err != nil {
		slog.Error("序列化响应失败", "err", err)
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	fmt.Println(string(respBytes))
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
	result.ServerInfo.Version = s.version

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

	start := time.Now()

	resultText, err := handler(params.Arguments)

	// token 统计埋点
	if s.tracker != nil && err == nil {
		duration := time.Since(start)
		if recordErr := s.tracker.Record(tokenstats.ToolCallRecord{
			ToolName:   params.Name,
			Args:       params.Arguments,
			OutputText: resultText,
			DurationMs: duration.Milliseconds(),
			Timestamp:  start,
		}); recordErr != nil {
			slog.Warn("记录 token 统计失败", "err", recordErr)
		}
	}

	if err != nil {
		slog.Error("工具执行失败", "tool", params.Name, "err", err)
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
