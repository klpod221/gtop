package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"gtop/internal/agent" // To use AgentPayload format
	"gtop/internal/collector"
	"gtop/internal/config"
	"gtop/web"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for the UI since it might be served locally
	},
}

type Server struct {
	port       int
	configPath string
	clients    map[*websocket.Conn]bool
	clientsMu  sync.Mutex
	intelCol   *collector.IntelGPUCollector
}

func NewServer(port int, configPath string) *Server {
	return &Server{
		port:       port,
		configPath: configPath,
		clients:    make(map[*websocket.Conn]bool),
	}
}

// Start initialises the collectors, starts the HTTP server and the broadcast loop.
func (s *Server) Start() error {
	// Initialize GPU collectors (best-effort)
	var err error
	s.intelCol, err = collector.NewIntelGPUCollector()
	if err != nil && s.intelCol == nil {
		log.Printf("Intel GPU: %v\n", err)
	}
	if s.intelCol != nil {
		defer s.intelCol.Close()
	}

	if nvidiaErr := collector.InitNvidia(); nvidiaErr != nil {
		log.Printf("WARN NVIDIA GPU: %v\n", nvidiaErr)
	} else {
		defer collector.ShutdownNvidia()
	}

	// CPU needs baseline measurement
	collector.CollectCPUStats()
	if s.intelCol != nil {
		s.intelCol.Collect()
	}

	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.wsHandler)

	// Config API endpoints
	mux.HandleFunc("/api/config", s.configHandler)

	// Serve static frontend files on "/"
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("failed to load embedded UI files: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(distFS)))

	// Start broadcast loop
	go s.broadcastLoop()

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting Web UI Server on http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

// configHandler handles GET and POST for /api/config (web section only).
func (s *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cfgPath := s.configPath
	if cfgPath == "" {
		var err error
		cfgPath, err = config.DefaultConfigPath()
		if err != nil {
			http.Error(w, "cannot determine config path", http.StatusInternalServerError)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		cfg, err := config.Load(cfgPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg.Web)

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "reading body: "+err.Error(), http.StatusBadRequest)
			return
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var webCfg config.WebConfig
		if err := json.Unmarshal(body, &webCfg); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Preserve web port from current config if not provided
		if webCfg.Port == 0 {
			webCfg.Port = cfg.Web.Port
		}
		cfg.Web = webCfg

		if err := config.Write(cfgPath, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	// Read loop to keep connection alive and handle client disconnects
	go func() {
		defer func() {
			s.clientsMu.Lock()
			delete(s.clients, conn)
			s.clientsMu.Unlock()
			conn.Close()
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

func (s *Server) broadcastLoop() {
	ticker := time.NewTicker(time.Second) // 1 second update interval
	defer ticker.Stop()

	for range ticker.C {
		payload := s.collectMetrics()
		data, err := json.Marshal(payload)
		if err != nil {
			log.Printf("JSON marshal error: %v", err)
			continue
		}

		s.clientsMu.Lock()
		for conn := range s.clients {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				conn.Close()
				delete(s.clients, conn)
			}
		}
		s.clientsMu.Unlock()
	}
}

// collectMetrics gathers current state and maps to agent.AgentPayload
func (s *Server) collectMetrics() agent.AgentPayload {
	payload := agent.AgentPayload{
		Timestamp: time.Now().UnixMilli(),
	}

	// Host
	h := collector.CollectHostInfo()
	payload.Host = &h

	// CPU
	cpu, _ := collector.CollectCPUStats()
	payload.CPU = &cpu

	// Memory
	mem, _ := collector.CollectMem()
	payload.Memory = &mem

	// Disk
	payload.DisksSpace = collector.CollectDisksSpace()
	payload.DisksIO = collector.CollectDisksIO()

	// Network
	payload.Network = collector.CollectNetwork()

	// Processes
	procs := collector.CollectProcesses(mem.Total)
	payload.Processes = procs

	// GPUs
	if s.intelCol != nil {
		stats := s.intelCol.Collect()
		if len(stats.Engines) > 0 || stats.FreqActMHz > 0 {
			payload.IntelGPU = &stats
		}
	}
	nv, _ := collector.CollectNvidia()
	if len(nv) > 0 {
		payload.NvidiaGPUs = nv
	}
	amd := collector.CollectAmd()
	if len(amd) > 0 {
		payload.AmdGPUs = amd
	}

	return payload
}
