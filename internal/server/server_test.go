package server

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/handy-h/code-context-mcp/internal/config"
)

func TestNewMCPServer(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	if srv == nil {
		t.Fatal("NewMCPServer returned nil")
	}
	if len(srv.tools) != 0 {
		t.Errorf("new server has %d tools, want 0", len(srv.tools))
	}
}

func TestRegisterTool(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	srv.RegisterTool("test_tool", func(args map[string]interface{}) (string, error) {
		return "ok", nil
	})
	if len(srv.tools) != 1 {
		t.Errorf("after register, tools count = %d, want 1", len(srv.tools))
	}
}

func TestHandleRequest_Initialize(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "initialize",
	}
	resp := srv.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", resp.JSONRPC)
	}
}

func TestHandleRequest_ToolsList(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "tools/list",
	}
	resp := srv.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("tools/list error: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("tools/list result is nil")
	}
}

func TestHandleRequest_ToolsCall_Success(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	srv.RegisterTool("echo", func(args map[string]interface{}) (string, error) {
		msg, _ := args["msg"].(string)
		return msg, nil
	})

	params, _ := json.Marshal(map[string]interface{}{
		"name":      "echo",
		"arguments": map[string]interface{}{"msg": "hello"},
	})
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "tools/call",
		Params:  params,
	}
	resp := srv.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("tools/call error: %s", resp.Error.Message)
	}
}

func TestHandleRequest_ToolsCall_NotFound(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	params, _ := json.Marshal(map[string]interface{}{
		"name":      "nonexistent",
		"arguments": map[string]interface{}{},
	})
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "tools/call",
		Params:  params,
	}
	resp := srv.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		t.Fatal("result should not be nil for tool-not-found")
	}
}

func TestHandleRequest_ToolsCall_Error(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	srv.RegisterTool("fail", func(args map[string]interface{}) (string, error) {
		return "", fmt.Errorf("tool failed")
	})

	params, _ := json.Marshal(map[string]interface{}{
		"name":      "fail",
		"arguments": map[string]interface{}{},
	})
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "tools/call",
		Params:  params,
	}
	resp := srv.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
}

func TestHandleRequest_Ping(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "ping",
	}
	resp := srv.handleRequest(req)
	if resp.Error != nil {
		t.Fatalf("ping error: %s", resp.Error.Message)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want 2.0", resp.JSONRPC)
	}
}

func TestHandleRequest_UnknownMethod(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "unknown_method",
	}
	resp := srv.handleRequest(req)
	if resp.Error == nil {
		t.Fatal("unknown method should return error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
}

func TestHandleRequest_InvalidParams(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"1"`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{invalid json`),
	}
	resp := srv.handleRequest(req)
	if resp.Error == nil {
		t.Fatal("invalid params should return error")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("error code = %d, want -32602", resp.Error.Code)
	}
}

func TestHandleRequest_Notification(t *testing.T) {
	srv := NewMCPServer(config.Config{}, "test")
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	resp := srv.handleRequest(req)
	if resp.JSONRPC != "" {
		t.Error("notification response should be empty (no JSONRPC)")
	}
}

func TestGetToolDefinitions(t *testing.T) {
	defs := GetToolDefinitions()
	if len(defs) != 6 {
		t.Errorf("GetToolDefinitions() returned %d tools, want 6", len(defs))
	}

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Name] = true
		if d.Description == "" {
			t.Errorf("tool %q has empty description", d.Name)
		}
	}

	expected := []string{"code_search", "file_context", "index_project", "symbol_search", "impact_analysis", "token_stats"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("tool %q not found in definitions", name)
		}
	}
}
