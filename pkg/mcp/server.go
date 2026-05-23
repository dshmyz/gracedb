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

// Transport handles MCP message framing.
// MCP uses newline-delimited JSON (one JSON-RPC message per line).
type Transport interface {
	// Read reads one JSON-RPC request message (a complete line).
	Read() ([]byte, error)
	// Write writes one JSON-RPC response message (with newline terminator).
	Write([]byte) error
	// Close closes the transport.
	Close() error
}

// StdioTransport reads from stdin and writes to stdout.
type StdioTransport struct {
	scanner *bufio.Scanner
	enc     *json.Encoder
}

// NewStdioTransport creates a transport over os.Stdin / os.Stdout.
func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		scanner: bufio.NewScanner(os.Stdin),
		enc:     json.NewEncoder(os.Stdout),
	}
}

func (t *StdioTransport) Read() ([]byte, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return t.scanner.Bytes(), nil
}

func (t *StdioTransport) Write(data []byte) error {
	// Use the encoder just for newline-terminated output
	_, err := os.Stdout.Write(data)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write([]byte("\n"))
	return err
}

func (t *StdioTransport) Close() error {
	return nil
}

// PipeTransport reads from and writes to an io.ReadWriter.
// Used for testing and in-process communication.
type PipeTransport struct {
	rw      io.ReadWriter
	scanner *bufio.Scanner
}

// NewPipeTransport creates a transport over a ReadWriter (e.g., in-memory pipe).
func NewPipeTransport(rw io.ReadWriter) *PipeTransport {
	return &PipeTransport{
		rw:      rw,
		scanner: bufio.NewScanner(rw),
	}
}

func (t *PipeTransport) Read() ([]byte, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	return t.scanner.Bytes(), nil
}

func (t *PipeTransport) Write(data []byte) error {
	_, err := t.rw.Write(data)
	if err != nil {
		return err
	}
	_, err = t.rw.Write([]byte("\n"))
	return err
}

func (t *PipeTransport) Close() error {
	if c, ok := t.rw.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

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

// Serve runs the MCP server over the given transport.
// Reads JSON-RPC requests, dispatches them, and writes responses.
// Returns when the transport returns an error, io.EOF, or ctx is cancelled.
func (s *Server) Serve(ctx context.Context, t Transport) error {
	defer t.Close()

	for {
		data, err := t.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}

		if len(data) == 0 {
			continue
		}

		var req map[string]any
		if err := json.Unmarshal(data, &req); err != nil {
			resp := makeError(nil, -32700, "parse error")
			if writeErr := t.Write(toJSON(resp)); writeErr != nil {
				return writeErr
			}
			continue
		}

		resp, err := s.HandleRequest(ctx, req)
		if err != nil {
			resp = makeError(req["id"], -32603, err.Error())
		}
		if err := t.Write(toJSON(resp)); err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return err
			}
		}
	}
}

// RunStdio runs the MCP server over stdin/stdout.
// Convenience wrapper around Serve with StdioTransport.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.Serve(ctx, NewStdioTransport())
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
	s.mu.RLock()
	toolCount := len(s.tools)
	s.mu.RUnlock()

	caps := map[string]any{
		"tools": map[string]any{},
	}
	if toolCount > 0 {
		caps["tools"] = map[string]any{}
	}

	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    caps,
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

// formatResult marshals a result to pretty-printed JSON, or falls back to
// fmt.Sprintf if marshalling fails.
func formatResult(result any) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", result)
	}
	return string(data)
}

// toJSON marshals a value to compact JSON bytes. Cannot fail for the map
// types we produce internally, so we ignore the error.
func toJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
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
