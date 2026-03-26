package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gtop/internal/agent"
	"gtop/internal/collector"
)

// Server implements a minimal MCP (Model Context Protocol) via Stdio over JSON-RPC 2.0
type Server struct {
	scanner  *bufio.Scanner
	intelCol *collector.IntelGPUCollector
}

type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewServer() *Server {
	return &Server{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

func (s *Server) Start() error {
	// Initialize collectors
	var err error
	s.intelCol, err = collector.NewIntelGPUCollector()
	if err != nil && s.intelCol == nil {
		fmt.Fprintf(os.Stderr, "Intel GPU: %v\n", err)
	}
	if s.intelCol != nil {
		defer s.intelCol.Close()
	}
	if nvidiaErr := collector.InitNvidia(); nvidiaErr != nil {
		fmt.Fprintf(os.Stderr, "NVIDIA GPU: %v\n", nvidiaErr)
	} else {
		defer collector.ShutdownNvidia()
	}
	
	collector.CollectCPUStats()
	if s.intelCol != nil {
		s.intelCol.Collect()
	}

	// Disable standard logger to avoid polluting stdout (MCP uses stdout for RPC)
	log.SetOutput(os.Stderr)

	for s.scanner.Scan() {
		line := s.scanner.Text()
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}
		s.handleRequest(req)
	}

	return s.scanner.Err()
}

func (s *Server) handleRequest(req Request) {
	switch req.Method {
	case "initialize":
		s.sendResult(req.Id, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    "gtop",
				"version": "1.0.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
		})
	case "notifications/initialized":
		// No response needed for notifications
	case "tools/list":
		s.sendResult(req.Id, map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "get_metrics",
					"description": "Get real-time system metrics (CPU, Memory, Disk, Network, Process, GPU)",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			},
		})
	case "tools/call":
		var params struct {
			Name string `json:"name"`
		}
		json.Unmarshal(req.Params, &params)

		if params.Name == "get_metrics" {
			payload := s.collectMetrics()
			data, _ := json.Marshal(payload)
			s.sendResult(req.Id, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": string(data),
					},
				},
			})
		} else {
			s.sendError(req.Id, -32601, "Tool not found")
		}
	default:
		s.sendError(req.Id, -32601, "Method not found")
	}
}

func (s *Server) sendResult(id json.RawMessage, result interface{}) {
	if len(id) == 0 {
		return // Notification, no id
	}
	s.sendResponse(Response{
		Jsonrpc: "2.0",
		Id:      id,
		Result:  result,
	})
}

func (s *Server) sendError(id json.RawMessage, code int, msg string) {
	if len(id) == 0 {
		return
	}
	s.sendResponse(Response{
		Jsonrpc: "2.0",
		Id:      id,
		Error: &Error{
			Code:    code,
			Message: msg,
		},
	})
}

func (s *Server) sendResponse(res Response) {
	data, _ := json.Marshal(res)
	fmt.Println(string(data))
}

func (s *Server) collectMetrics() agent.AgentPayload {
	payload := agent.AgentPayload{}
	h := collector.CollectHostInfo()
	payload.Host = &h
	cpu, _ := collector.CollectCPUStats()
	payload.CPU = &cpu
	mem, _ := collector.CollectMem()
	payload.Memory = &mem
	payload.DisksSpace = collector.CollectDisksSpace()
	payload.DisksIO = collector.CollectDisksIO()
	payload.Network = collector.CollectNetwork()
	procs := collector.CollectProcesses(mem.Total)
	
	// Limit processes to top 20 to avoid exceeding LLM context
	if len(procs) > 20 {
		procs = procs[:20]
	}
	payload.Processes = procs

	if s.intelCol != nil {
		stats := s.intelCol.Collect()
		payload.IntelGPU = &stats
	}
	nv, _ := collector.CollectNvidia()
	payload.NvidiaGPUs = nv
	amd := collector.CollectAmd()
	payload.AmdGPUs = amd

	return payload
}
