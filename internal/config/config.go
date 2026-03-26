package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultConfigDir = ".config/gtop"
const DefaultConfigFile = "config.json"

// ServerConfig holds all HTTP transport settings for the remote endpoint.
type ServerConfig struct {
	// Endpoint is the full URL to POST telemetry data to.
	Endpoint string `json:"endpoint"`

	// AuthToken is the value set in the auth header. Leave empty to skip auth.
	AuthToken string `json:"auth_token"`

	// AuthHeader is the HTTP header name used to send the token.
	// Defaults to "Authorization". Common alternatives: "X-Api-Key", "X-Token".
	AuthHeader string `json:"auth_header"`

	// TimeoutSeconds is the HTTP request timeout in seconds.
	TimeoutSeconds int `json:"timeout_seconds"`

	// RetryCount is how many times to retry a failed HTTP request.
	RetryCount int `json:"retry_count"`

	// RetryDelaySeconds is the base delay before the first retry.
	// Each subsequent retry doubles the delay (exponential backoff).
	RetryDelaySeconds int `json:"retry_delay_seconds"`

	// TLSSkipVerify disables TLS certificate verification. Use only for testing.
	TLSSkipVerify bool `json:"tls_skip_verify"`

	// TLSCACert is the path to a custom PEM-encoded CA certificate file.
	// Useful for servers using self-signed or private CA certificates.
	TLSCACert string `json:"tls_ca_cert"`

	// Compress enables gzip compression of the request body before sending.
	Compress bool `json:"compress"`
}

// AgentBehaviorConfig controls the agent daemon's runtime behavior.
type AgentBehaviorConfig struct {
	// IntervalSeconds is the data collection and send cycle in seconds.
	IntervalSeconds int `json:"interval_seconds"`

	// MachineID is a stable unique identifier for this machine sent in every payload.
	// If empty, it is auto-generated from /etc/machine-id on first run.
	MachineID string `json:"machine_id"`

	// MachineName is a human-friendly name for this machine.
	// Defaults to the system hostname if empty.
	MachineName string `json:"machine_name"`

	// Tags are arbitrary key-value pairs appended to every payload.
	// Useful for grouping machines by location, role, environment, etc.
	Tags map[string]string `json:"tags"`

	// LogLevel controls verbosity: "debug", "info", "warn", "error".
	LogLevel string `json:"log_level"`

	// LogFile is the path to a log file. If empty, logs go to stderr.
	LogFile string `json:"log_file"`

	// PIDFile is the path where the agent writes its PID when running.
	PIDFile string `json:"pid_file"`
}

// WebConfig holds settings for the embedded Web UI server.
type WebConfig struct {
	// Port is the HTTP port the web server listens on.
	Port int `json:"port"`

	// NetworkInterface is the preferred interface to show on the topbar.
	// If empty, auto-selects by priority: LAN (en*) > WiFi (wl*) > first other.
	NetworkInterface string `json:"network_interface"`

	// StorageFilter restricts which mount points are shown in the UI.
	// An empty list means all mounts are shown.
	StorageFilter []string `json:"storage_filter"`
}

// CPUModuleConfig controls what CPU data is collected.
type CPUModuleConfig struct {
	// Enabled toggles the entire CPU module.
	Enabled bool `json:"enabled"`

	// Fields selects which CPU metrics to include in the payload.
	// Valid values: "usage", "freq", "temp", "power", "loadavg", "uptime", "name", "battery".
	// An empty list means all fields are included.
	Fields []string `json:"fields"`
}

// DiskModuleConfig controls what disk data is collected.
type DiskModuleConfig struct {
	// Enabled toggles the entire disk module.
	Enabled bool `json:"enabled"`

	// MountFilter restricts collection to the listed mount points.
	// An empty list means all mounted filesystems are collected.
	MountFilter []string `json:"mount_filter"`
}

// NetworkModuleConfig controls network interface data collection.
type NetworkModuleConfig struct {
	// Enabled toggles the entire network module.
	Enabled bool `json:"enabled"`

	// IfaceFilter restricts collection to the listed interface names (e.g. "eth0", "wlan0").
	// An empty list means all interfaces are collected.
	IfaceFilter []string `json:"iface_filter"`

	// ExcludeVirtual filters out virtual interfaces such as Docker bridges (br-*),
	// veth pairs, and loopback when set to true.
	ExcludeVirtual bool `json:"exclude_virtual"`
}

// ProcessModuleConfig controls process list collection.
type ProcessModuleConfig struct {
	// Enabled toggles the process module. Disabled by default for performance.
	Enabled bool `json:"enabled"`

	// TopN limits the number of processes included in the payload (sorted by SortBy).
	// 0 means unlimited.
	TopN int `json:"top_n"`

	// SortBy determines the sort order: "cpu", "mem", "pid", "name", "io".
	SortBy string `json:"sort_by"`

	// NameFilter retains only processes whose name or cmdline contains this substring.
	// Case-insensitive. Empty string disables filtering.
	NameFilter string `json:"name_filter"`
}

// GPUModuleConfig controls GPU collection per vendor.
type GPUModuleConfig struct {
	// Enabled toggles the entire GPU module.
	Enabled bool `json:"enabled"`

	// Intel enables Intel GPU metrics via perf counters.
	Intel bool `json:"intel"`

	// Nvidia enables NVIDIA GPU metrics via NVML.
	Nvidia bool `json:"nvidia"`

	// AMD enables AMD GPU metrics via sysfs.
	AMD bool `json:"amd"`
}

// ModulesConfig groups per-module enable/filter settings.
type ModulesConfig struct {
	Host      SimpleModuleConfig  `json:"host"`
	CPU       CPUModuleConfig     `json:"cpu"`
	Memory    SimpleModuleConfig  `json:"memory"`
	Disk      DiskModuleConfig    `json:"disk"`
	Network   NetworkModuleConfig `json:"network"`
	Processes ProcessModuleConfig `json:"processes"`
	GPU       GPUModuleConfig     `json:"gpu"`
}

// SimpleModuleConfig is used for modules with only an enable toggle.
type SimpleModuleConfig struct {
	Enabled bool `json:"enabled"`
}

// Config is the root configuration structure for gtop.
// It is loaded from $HOME/.config/gtop/config.json.
type Config struct {
	// EnabledAgent enables the telemetry agent daemon.
	EnabledAgent bool `json:"enabled_agent"`

	// EnabledWeb enables the embedded Web UI server.
	EnabledWeb bool `json:"enabled_web"`

	Web     WebConfig           `json:"web"`
	Server  ServerConfig        `json:"server"`
	Agent   AgentBehaviorConfig `json:"agent"`
	Modules ModulesConfig       `json:"modules"`
}

// AgentConfig is an alias kept for backwards-compatibility inside the codebase.
type AgentConfig = Config

// DefaultConfig returns a fully-populated config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		EnabledAgent: false,
		EnabledWeb:   false,
		Web: WebConfig{
			Port:             8080,
			NetworkInterface: "",
			StorageFilter:    []string{},
		},
		Server: ServerConfig{
			Endpoint:          "http://your-server:8080/api/telemetry",
			AuthToken:         "",
			AuthHeader:        "Authorization",
			TimeoutSeconds:    10,
			RetryCount:        3,
			RetryDelaySeconds: 5,
			TLSSkipVerify:     false,
			TLSCACert:         "",
			Compress:          false,
		},
		Agent: AgentBehaviorConfig{
			IntervalSeconds: 5,
			MachineID:       "",
			MachineName:     "",
			Tags:            map[string]string{},
			LogLevel:        "info",
			LogFile:         "",
			PIDFile:         "/tmp/gtop-agent.pid",
		},
		Modules: ModulesConfig{
			Host:   SimpleModuleConfig{Enabled: true},
			Memory: SimpleModuleConfig{Enabled: true},
			CPU: CPUModuleConfig{
				Enabled: true,
				Fields:  []string{"usage", "freq", "temp", "power", "loadavg", "uptime", "name"},
			},
			Disk: DiskModuleConfig{
				Enabled:     true,
				MountFilter: []string{},
			},
			Network: NetworkModuleConfig{
				Enabled:        true,
				IfaceFilter:    []string{},
				ExcludeVirtual: false,
			},
			Processes: ProcessModuleConfig{
				Enabled:    false,
				TopN:       20,
				SortBy:     "cpu",
				NameFilter: "",
			},
			GPU: GPUModuleConfig{
				Enabled: true,
				Intel:   true,
				Nvidia:  true,
				AMD:     true,
			},
		},
	}
}

// DefaultConfigPath returns the default config file path using $HOME.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, DefaultConfigDir, DefaultConfigFile), nil
}

// Load reads and parses the config from the given file path.
// If the file does not exist, it writes a default config and returns it.
func Load(path string) (Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if writeErr := Write(path, cfg); writeErr != nil {
			return cfg, fmt.Errorf("config not found; also failed to write default: %w", writeErr)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if err := Validate(cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Write serializes cfg to JSON and writes it to path, creating parent directories.
func Write(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// Validate checks the config for required fields and sane values.
func Validate(cfg Config) error {
	if cfg.Agent.IntervalSeconds < 1 {
		return fmt.Errorf("agent.interval_seconds must be >= 1")
	}
	if cfg.Server.TimeoutSeconds < 1 {
		return fmt.Errorf("server.timeout_seconds must be >= 1")
	}
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Agent.LogLevel] {
		return fmt.Errorf("agent.log_level must be one of: debug, info, warn, error")
	}
	validSortBy := map[string]bool{"cpu": true, "mem": true, "pid": true, "name": true, "io": true}
	if cfg.Modules.Processes.SortBy != "" && !validSortBy[cfg.Modules.Processes.SortBy] {
		return fmt.Errorf("modules.processes.sort_by must be one of: cpu, mem, pid, name, io")
	}
	if cfg.Web.Port < 1 || cfg.Web.Port > 65535 {
		return fmt.Errorf("web.port must be between 1 and 65535")
	}
	return nil
}
