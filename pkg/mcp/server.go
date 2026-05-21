// Package mcp provides a simplified Model Context Protocol server.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// Tool represents an MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Handler     ToolHandler
}

// ToolHandler handles tool invocation.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// Server is an MCP-compatible JSON-RPC server.
type Server struct {
	name    string
	version string
	tools   map[string]*Tool
	mu      sync.RWMutex
}

// NewServer creates a new MCP server.
func NewServer(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]*Tool),
	}
}

// RegisterTool adds a tool to the server.
func (s *Server) RegisterTool(tool *Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
}

// RegisterTools adds multiple tools.
func (s *Server) RegisterTools(tools []*Tool) {
	for _, t := range tools {
		s.RegisterTool(t)
	}
}

// HandleRequest processes a single MCP JSON-RPC request.
func (s *Server) HandleRequest(ctx context.Context, req map[string]any) (map[string]any, error) {
	method, _ := req["method"].(string)
	id, _ := req["id"]

	switch method {
	case "initialize":
		return s.handleInitialize(id)
	case "tools/list":
		return s.handleToolsList(id)
	case "tools/call":
		params, _ := req["params"].(map[string]any)
		return s.handleToolCall(ctx, id, params)
	default:
		return makeError(id, -32601, fmt.Sprintf("method not found: %s", method)), nil
	}
}

// RunStdio runs the MCP server over stdin/stdout.
func (s *Server) RunStdio(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req map[string]any
		if err := json.Unmarshal(line, &req); err != nil {
			resp := makeError(nil, -32700, "parse error")
			_ = encoder.Encode(resp)
			continue
		}

		resp, err := s.HandleRequest(ctx, req)
		if err != nil {
			resp = makeError(req["id"], -32603, err.Error())
		}
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// ToolDef describes a tool for MCP registration.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// FromToolbox creates an MCP server from a toolbox.
func FromToolbox(name, version string, tools []ToolDef, call func(ctx context.Context, name string, args map[string]any) (any, error)) *Server {
	s := NewServer(name, version)
	for _, td := range tools {
		tool := &Tool{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				return call(ctx, td.Name, args)
			},
		}
		s.RegisterTool(tool)
	}
	return s
}

func (s *Server) handleInitialize(id any) (map[string]any, error) {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"serverInfo": map[string]any{
				"name":    s.name,
				"version": s.version,
			},
		},
	}, nil
}

func (s *Server) handleToolsList(id any) (map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}

	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]any{"tools": tools},
	}, nil
}

func (s *Server) handleToolCall(ctx context.Context, id any, params map[string]any) (map[string]any, error) {
	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]any)

	s.mu.RLock()
	tool, ok := s.tools[name]
	s.mu.RUnlock()

	if !ok {
		return makeError(id, -32602, fmt.Sprintf("unknown tool: %s", name)), nil
	}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		return makeError(id, -32603, err.Error()), nil
	}

	content := []map[string]any{
		{
			"type": "text",
			"text": formatResult(result),
		},
	}

	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"content": content,
			"isError": false,
		},
	}, nil
}

func formatResult(result any) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(data)
}

func makeError(id any, code int, msg string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	}
}
