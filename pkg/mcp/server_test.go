package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestServer_HandleInitialize(t *testing.T) {
	s := NewServer("test-srv", "1.0.0")
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("handle initialize: %v", err)
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object")
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %v", result["protocolVersion"])
	}

	si, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("expected serverInfo object")
	}
	if si["name"] != "test-srv" {
		t.Errorf("expected server name 'test-srv', got %v", si["name"])
	}
	if si["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", si["version"])
	}
}

func TestServer_HandleToolsList(t *testing.T) {
	s := NewServer("test-srv", "1.0.0")
	s.RegisterTool(&Tool{
		Name:        "echo",
		Description: "Echo back the input",
		InputSchema: map[string]any{"text": "string"},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return args, nil
		},
	})

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("handle tools/list: %v", err)
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object")
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatalf("expected tools array")
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0]["name"] != "echo" {
		t.Errorf("expected tool name 'echo', got %v", tools[0]["name"])
	}
}

func TestServer_HandleToolCall_Success(t *testing.T) {
	s := NewServer("test-srv", "1.0.0")
	s.RegisterTool(&Tool{
		Name:        "echo",
		Description: "Echo",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return args["text"], nil
		},
	})

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "echo",
			"arguments": map[string]any{
				"text": "hello",
			},
		},
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("handle tools/call: %v", err)
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object")
	}

	content, ok := result["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected content array")
	}
	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}
	if content[0]["type"] != "text" {
		t.Errorf("expected content type 'text', got %v", content[0]["type"])
	}
}

func TestServer_HandleToolCall_UnknownTool(t *testing.T) {
	s := NewServer("test-srv", "1.0.0")

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "unknown",
		},
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("handle call: %v", err)
	}

	if resp["error"] == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestServer_HandleToolCall_HandlerError(t *testing.T) {
	s := NewServer("test-srv", "1.0.0")
	s.RegisterTool(&Tool{
		Name: "failing",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return nil, io.EOF
		},
	})

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "failing",
		},
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("handle call: %v", err)
	}

	if resp["error"] == nil {
		t.Fatal("expected error from failing handler")
	}
}

func TestServer_HandleUnknownMethod(t *testing.T) {
	s := NewServer("test-srv", "1.0.0")

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "unknown/method",
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("handle request: %v", err)
	}

	if resp["error"] == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestServer_FromToolbox(t *testing.T) {
	call := func(ctx context.Context, name string, args map[string]any) (any, error) {
		return map[string]any{"name": name, "args": args}, nil
	}

	defs := []ToolDef{
		{Name: "tool_a", Description: "Tool A", InputSchema: map[string]any{}},
		{Name: "tool_b", Description: "Tool B", InputSchema: map[string]any{}},
	}

	s := FromToolbox("tbx", "1.0", defs, call)
	if s == nil {
		t.Fatal("FromToolbox returned nil")
	}

	// Test tool call through the server
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "tool_a",
			"arguments": map[string]any{
				"key": "value",
			},
		},
	}

	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("handle call: %v", err)
	}

	result := resp["result"].(map[string]any)
	content := result["content"].([]map[string]any)
	text := content[0]["text"].(string)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if parsed["name"] != "tool_a" {
		t.Errorf("expected name 'tool_a', got %v", parsed["name"])
	}
}

func TestServer_RunStdio(t *testing.T) {
	s := NewServer("stdio-test", "1.0")
	s.RegisterTool(&Tool{
		Name: "ping",
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return "pong", nil
		},
	})

	// Bidirectional pipe pair: client writes go to server, server writes go to client
	serverR, clientW := io.Pipe() // server reads from serverR, client writes to clientW
	clientR, serverW := io.Pipe() // client reads from clientR, server writes to serverW

	serverRW := struct {
		io.Reader
		io.Writer
	}{serverR, serverW}
	clientRW := struct {
		io.Reader
		io.Writer
	}{clientR, clientW}

	serverTransport := NewPipeTransport(serverRW)
	clientTransport := NewPipeTransport(clientRW)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(context.Background(), serverTransport)
	}()

	// Send request from client side
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"ping"}}` + "\n"
	if _, err := clientRW.Write([]byte(req)); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// Read response
	respData, err := clientTransport.Read()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(respData, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	first := content[0].(map[string]any)
	if first["text"] != "\"pong\"" {
		t.Errorf("expected '\"pong\"', got %v", first["text"])
	}

	// Close client's write side to signal EOF to server, then verify Serve returns cleanly
	clientW.Close()
	if err := <-errCh; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestPipeTransport(t *testing.T) {
	var buf bytes.Buffer
	tr := NewPipeTransport(&buf)

	// Write and read through the same transport (unidirectional test)
	if err := tr.Write([]byte(`{"msg":"hello"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	msg, err := tr.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(msg) != `{"msg":"hello"}` {
		t.Errorf("expected hello, got %s", msg)
	}

	// Second message
	if err := tr.Write([]byte(`{"msg":"world"}`)); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	msg2, err := tr.Read()
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	if string(msg2) != `{"msg":"world"}` {
		t.Errorf("expected world, got %s", msg2)
	}
}

func TestServer_RegisterTools(t *testing.T) {
	s := NewServer("multi", "1.0")
	tools := []*Tool{
		{Name: "a", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "a", nil }},
		{Name: "b", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "b", nil }},
		{Name: "c", Handler: func(ctx context.Context, args map[string]any) (any, error) { return "c", nil }},
	}
	s.RegisterTools(tools)

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	resp, err := s.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}

	result := resp["result"].(map[string]any)
	toolsList := result["tools"].([]map[string]any)
	if len(toolsList) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(toolsList))
	}
}
