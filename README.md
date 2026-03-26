# gtop

A high-performance, Linux-native system telemetry agent written in Go with a **TUI dashboard**.  
Zero external shell dependencies — all data is collected by directly parsing kernel interfaces (`/proc`, `/sys`, `perf_event_open`).

## Build & Run

```bash
# Build
go build -o gtop .

# With Intel GPU monitoring (requires capabilities, see below)
sudo setcap cap_perfmon,cap_dac_read_search=ep ./gtop

# Launch TUI dashboard (default)
./gtop

# Run CLI mode (JSON output)
./gtop get
```

## TUI Dashboard

The interactive terminal dashboard is the default execution mode.

```bash
./gtop
```

### Layout

```
┌──── CPU Graph (sparkline) ────┬── CPU Cores (bars+info) ──┐
│ total usage over time         │ C0██ C6██  i5-10400       │
│                               │ C1██ C7██  3.5 GHz 45°C  │
├──── mem ──────┬── disks ──────┼──── proc ─────────────────┤
│ Total: 15.4G  │ root ████ 81%│ PID User Name CPU% MEM%   │
│ Used:  8.0G   │ home ████ 2% │ 1   root systemd  0.1 1.2 │
├──── gpu ──────┴───────────────┤                           │
│ Intel GPU  Freq: 350 MHz     │                           │
│ rcs0 ████░░ 12%              │                           │
├──── net ──────────────────────┤                           │
│ ▁▂▃▅▇█▅▃▂ ↓ 1.4 KiB/s       │                           │
│ ▁▁▁▂▁▁▁▁  ↑ 132 B/s         │                           │
└───────────────────────────────┴───────────────────────────┘
```

### Keybindings

| Key | Action |
|-----|--------|
| `q` | Quit |
| `Ctrl+C` | Quit |

### Capabilities (setcap)

The `setcap` command grants **two specific Linux capabilities** to the binary:

| Capability | Purpose |
|-----------|---------|
| `cap_perfmon` | Access `perf_event_open()` syscall for Intel GPU PMU counters without root |
| `cap_dac_read_search` | Read restricted sysfs/procfs files (e.g. RAPL energy counters) |

**This is NOT sudo** — it grants only these two specific permissions, permanently attached to the binary file. You run it once after building, then `./gtop` works as a normal user.

**Without setcap:**
- CPU, Memory, Disk, Network, Processes → **work normally** ✅
- Intel GPU metrics → **will fail** with `perf_event_open: operation not permitted` ❌
- An error message + hint will be printed to stderr

---

## CLI Subcommands

### Mode

| Command | Description | Example |
|------|-------------|---------|
| `gtop` / `gtop tui` | Launch interactive TUI dashboard | `./gtop` |
| `gtop get` | Fetch and export telemetry data (JSON/Flat) | `./gtop get --modules cpu` |
| `gtop agent` | Run as background telemetry daemon | `./gtop agent --dry-run --once` |
| `gtop web` | *(Future)* Launch web-based UI | `./gtop web` |
| `gtop mcp` | *(Future)* Launch Model Context Protocol server | `./gtop mcp` |

### `gtop get` Flags (CLI mode)

| Flag | Default | Description | Example |
|------|---------|-------------|---------|
| `--modules` | `all` | Modules to collect (comma-separated) | `--modules cpu,mem,gpu` |
| `--no-proc` | `false` | Skip process collection (lighter output) | `--no-proc` |

Valid modules: `cpu`, `mem`, `disk`, `net`, `proc`, `gpu`, `all`

### Output Control

| Flag | Default | Description | Example |
|------|---------|-------------|---------|
| `--output` | stdout | Write JSON to file instead of stdout | `--output /tmp/stats.json` |
| `--format` | `json` | Output format: `json` or `flat` | `--format flat` |
| `--compact` | `false` | No indentation (single-line JSON) | `--compact` |
| `--interval` | `1000` | Collection interval in milliseconds | `--interval 2000` |
| `--count` | `1` | Number of collection cycles (0 = infinite) | `--count 0` |

### CPU Filtering

| Flag | Default | Description | Example |
|------|---------|-------------|---------|
| `--cpu-fields` | all | Filter CPU output fields (comma-separated) | `--cpu-fields usage,power,temp` |

Valid fields: `usage`, `freq`, `temp`, `power`, `loadavg`, `uptime`, `name`, `battery`

### Process Filtering

| Flag | Default | Description | Example |
|------|---------|-------------|---------|
| `--proc-sort` | `cpu` | Sort processes by: `cpu`, `mem`, `pid`, `name`, `io` | `--proc-sort mem` |
| `--proc-top` | `0` (all) | Limit number of processes returned | `--proc-top 20` |
| `--proc-filter` | none | Filter by name/cmdline substring (case-insensitive) | `--proc-filter firefox` |

### Network & Disk Filtering

| Flag | Default | Description | Example |
|------|---------|-------------|---------|
| `--net-iface` | all | Filter network interfaces (comma-separated) | `--net-iface enp1s0,wlan0` |
| `--disk-mount` | all | Filter disk mount points (comma-separated) | `--disk-mount /,/home` |

### Usage Examples

```bash
# Quick system overview without processes
./gtop get --modules cpu,mem --no-proc --compact

# Monitor CPU power consumption only
./gtop get --modules cpu --cpu-fields power --count 0 --interval 500

# Top 10 processes by memory, continuously
./gtop get --modules proc --proc-sort mem --proc-top 10 --count 0

# GPU monitoring, save to file
./gtop get --modules gpu --count 5 --output /tmp/gpu.json

# Full system snapshot, compact JSON for piping
./gtop get --compact | jq '.cpu.usage_percent'

# Monitor specific network interface
./gtop get --modules net --net-iface enp1s0 --count 0

# Filter processes matching "docker"
./gtop get --modules proc --proc-filter docker --proc-top 20
```

---

## Agent Service (Daemon Mode)

> **oikos-agent** — a long-running background agent that periodically collects system metrics and pushes them to a configurable remote endpoint.

### Quick Start

```bash
# 1. Test locally without sending anything (dry-run, single cycle)
./gtop agent --dry-run --once

# 2. Edit config (auto-created with defaults on first run)
nano ~/.config/oikos-agent/config.json

# 3. Run as a persistent daemon
./gtop agent

# 4. Install as a systemd service (system-wide, needs root)
sudo ./gtop agent install

# 5. Install as a user service (no root required)
./gtop agent install --user
```

### `gtop agent` Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.config/oikos-agent/config.json` | Override config file path |
| `--dry-run` | `false` | Collect metrics, print to stderr, do **not** send |
| `--once` | `false` | Run one collection cycle then exit |

### Agent Subcommands

| Subcommand | Description |
|------|-------------|
| `gtop agent install` | Write systemd unit + `systemctl enable --now gtop-agent` |
| `gtop agent install --user` | Same but as a user service (`~/.config/systemd/user/`) |
| `gtop agent uninstall` | `systemctl disable --now` + remove unit file |
| `gtop agent config` | Print current config (creates default if missing) |
| `gtop agent config --init` | (Re-)write default config then print it |
| `gtop agent status` | Check if daemon is running via PID file |

### Config File Reference

**Location:** `~/.config/oikos-agent/config.json` (auto-created with defaults)

```json
{
  "server": {
    "endpoint": "http://your-server:8080/api/telemetry",
    "auth_token": "",
    "auth_header": "Authorization",
    "timeout_seconds": 10,
    "retry_count": 3,
    "retry_delay_seconds": 5,
    "tls_skip_verify": false,
    "tls_ca_cert": "",
    "compress": false
  },
  "agent": {
    "interval_seconds": 5,
    "machine_id": "",
    "machine_name": "",
    "tags": {},
    "log_level": "info",
    "log_file": "",
    "pid_file": "/tmp/gtop-agent.pid"
  },
  "modules": {
    "host":    { "enabled": true },
    "memory":  { "enabled": true },
    "cpu": {
      "enabled": true,
      "fields": ["usage", "freq", "temp", "power", "loadavg", "uptime", "name"]
    },
    "disk": {
      "enabled": true,
      "mount_filter": []
    },
    "network": {
      "enabled": true,
      "iface_filter": [],
      "exclude_virtual": false
    },
    "processes": {
      "enabled": false,
      "top_n": 20,
      "sort_by": "cpu",
      "name_filter": ""
    },
    "gpu": {
      "enabled": true,
      "intel": true,
      "nvidia": true,
      "amd": true
    }
  }
}
```

#### `server` Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `endpoint` | string | — | **Required.** Full URL to POST telemetry JSON to |
| `auth_token` | string | `""` | Bearer token / API key value. Omitted if empty |
| `auth_header` | string | `"Authorization"` | HTTP header name for the token (e.g. `X-Api-Key`) |
| `timeout_seconds` | int | `10` | HTTP request timeout in seconds |
| `retry_count` | int | `3` | Retry attempts on network failure |
| `retry_delay_seconds` | int | `5` | Base delay between retries (doubles each attempt) |
| `tls_skip_verify` | bool | `false` | Skip TLS cert validation (dev/testing only) |
| `tls_ca_cert` | string | `""` | Path to custom PEM CA cert file |
| `compress` | bool | `false` | Gzip-compress the POST body |

#### `agent` Options

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `interval_seconds` | int | `5` | Collection + send cycle length |
| `machine_id` | string | auto | Stable machine identifier (auto-read from `/etc/machine-id`) |
| `machine_name` | string | hostname | Human-readable machine label |
| `tags` | object | `{}` | Arbitrary key-value metadata sent in every payload |
| `log_level` | string | `"info"` | Verbosity: `debug`, `info`, `warn`, `error` |
| `log_file` | string | `""` | Log file path. Empty = stderr |
| `pid_file` | string | `/tmp/gtop-agent.pid` | PID file written on start, removed on exit |

#### `modules` Options

| Module | Key | Options |
|--------|-----|---------|
| **host** | `enabled` | bool |
| **cpu** | `enabled`, `fields` | `fields`: array of `usage`, `freq`, `temp`, `power`, `loadavg`, `uptime`, `name`, `battery` |
| **memory** | `enabled` | bool |
| **disk** | `enabled`, `mount_filter` | `mount_filter`: only collect these mount points (empty = all) |
| **network** | `enabled`, `iface_filter`, `exclude_virtual` | `iface_filter`: filter by interface names; `exclude_virtual`: strip `br-*`, `veth*`, `docker*`, `lo` |
| **processes** | `enabled`, `top_n`, `sort_by`, `name_filter` | `sort_by`: `cpu`, `mem`, `pid`, `name`, `io`; disabled by default |
| **gpu** | `enabled`, `intel`, `nvidia`, `amd` | Toggle per-vendor collection |

### Payload Schema

The agent sends the same JSON payload as `gtop get`, enriched with agent metadata:

```json
{
  "timestamp": 1774425456106,
  "machine_id": "abc123...",
  "machine_name": "my-server",
  "tags": { "env": "prod", "rack": "A1" },
  "host": { ... },
  "cpu": { ... },
  "memory": { ... },
  "disks_space": [ ... ],
  "disks_io": [ ... ],
  "network": [ ... ],
  "intel_gpu": { ... }
}
```

### Systemd Service

After `gtop agent install`, the unit file is written to `/etc/systemd/system/gtop-agent.service`:

```ini
[Unit]
Description=gtop Telemetry Agent (oikos-agent)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/gtop agent
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gtop-agent

[Install]
WantedBy=multi-user.target
```

```bash
# View logs
journalctl -u gtop-agent -f

# Restart
systemctl restart gtop-agent

# Stop
systemctl stop gtop-agent
```

---

## Collected Data

### CPU Module (`cpu`)

**Source files:** `/proc/stat`, `/proc/uptime`, `/proc/loadavg`, `/proc/cpuinfo`, `/sys/class/powercap/`, `/sys/class/hwmon/`, `/sys/devices/system/cpu/`

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| Overall usage | `usage_percent` | % | Delta of `/proc/stat` `cpu` line (active ticks / total ticks × 100) |
| Per-core usage | `cores_percent` | % array | Delta of `/proc/stat` `cpuN` lines |
| Per-core frequency | `freq_mhz` | MHz array | `/sys/devices/system/cpu/cpuN/cpufreq/scaling_cur_freq` |
| Per-physical-core temps | `core_temps_c` | °C array | `/sys/class/hwmon/hwmonN/tempN_input` (coretemp driver) |
| Package temp | `package_temp_c` | °C | hwmon entry labeled `Package` or highest-numbered sensor |
| Load average | `load_avg` | [1m, 5m, 15m] | `/proc/loadavg` |
| System uptime | `uptime_seconds` | seconds | `/proc/uptime` |
| CPU package power | `power_watts` | Watts | Delta of `/sys/class/powercap/intel-rapl:0/energy_uj` ÷ delta time |
| CPU model name | `cpu_name` | string | `/proc/cpuinfo` → `model name` |
| Battery level | `battery_percent` | % | `/sys/class/power_supply/BAT*/capacity` |
| Battery status | `battery_status` | string | `/sys/class/power_supply/BAT*/status` |

**Delta calculation:** CPU usage requires two samples to compute meaningful percentages. The agent performs an automatic "prime read" before the main loop, so even `--count 1` returns accurate data.

---

### Memory Module (`mem`)

**Source files:** `/proc/meminfo`, `/proc/spl/kstat/zfs/arcstats`

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| Total RAM | `total` | bytes | `MemTotal` |
| Available RAM | `available` | bytes | `MemAvailable` (fallback: `Free + Cached`) |
| Used RAM | `used` | bytes | `Total - Available` |
| Free RAM | `free` | bytes | `MemFree` |
| Cached | `cached` | bytes | `Cached` + ZFS ARC size |
| Buffers | `buffers` | bytes | `Buffers` |
| Swap total | `swap_total` | bytes | `SwapTotal` |
| Swap free | `swap_free` | bytes | `SwapFree` |
| Swap used | `swap_used` | bytes | `SwapTotal - SwapFree` |
| ZFS ARC cache | `zfs_arc` | bytes | `/proc/spl/kstat/zfs/arcstats` → `size` |

---

### Disk Module (`disk`)

**Source files:** `/etc/mtab` or `/proc/self/mounts`, `/proc/filesystems`, `/sys/block/*/stat`

#### Disk Space (`disks_space`)

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| Mount point | `mount_point` | path | Mount list |
| Device | `device` | path | Mount list |
| Filesystem type | `fs_type` | string | Mount list |
| Display name | `name` | string | `basename(mount)` |
| Total | `total_bytes` | bytes | `statfs()` syscall |
| Used | `used_bytes` | bytes | `total - available` |
| Free | `free_bytes` | bytes | `statfs()` → `bavail` |
| Used percentage | `used_pct` | % | `used / total × 100` |

#### Disk I/O (`disks_io`)

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| Device name | `device` | string | Sysfs device name |
| Read bytes | `read_bytes` | bytes | `read_sectors × 512` |
| Write bytes | `write_bytes` | bytes | `write_sectors × 512` |
| Read IOPS | `read_iops` | count | Cumulative read I/O operations |
| Write IOPS | `write_iops` | count | Cumulative write I/O operations |
| I/O ticks | `io_ticks_ms` | ms | Time spent doing I/O |

---

### Network Module (`net`)

**Source files:** `/sys/class/net/*/`, `getifaddrs()` equivalent via Go `net` package

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| Interface name | `name` | string | Sysfs directory name |
| IPv4 address | `ipv4` | string | `net.InterfaceByName()` → `Addrs()` |
| IPv6 address | `ipv6` | string | Same |
| MAC address | `mac` | string | `/sys/class/net/{iface}/address` |
| RX bytes | `rx_bytes` | bytes | `/sys/class/net/{iface}/statistics/rx_bytes` |
| TX bytes | `tx_bytes` | bytes | `/sys/class/net/{iface}/statistics/tx_bytes` |
| Connected | `connected` | bool | `/sys/class/net/{iface}/operstate` == `up` |

---

### Process Module (`proc`)

**Source files:** `/proc/[pid]/stat`, `/proc/[pid]/status`, `/proc/[pid]/cmdline`, `/proc/[pid]/io`, `/etc/passwd`

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| PID | `pid` | int | Directory name |
| Parent PID | `ppid` | int | `/proc/[pid]/stat` field 4 |
| User ID | `uid` | int | `/proc/[pid]/status` → `Uid` |
| Username | `user` | string | `/etc/passwd` lookup by UID |
| Process name | `name` | string | `/proc/[pid]/stat` comm field (handles parens) |
| Command line | `cmdline` | string | `/proc/[pid]/cmdline` (null-byte separated → spaces) |
| State | `state` | string | R=Running, S=Sleeping, D=Disk, Z=Zombie, T=Stopped, I=Idle |
| Thread count | `threads` | int | `/proc/[pid]/stat` field 20 |
| RSS memory | `mem_rss_bytes` | bytes | `/proc/[pid]/status` → `VmRSS` × 1024 |
| Memory % | `mem_percent` | % | `RSS / TotalRAM × 100` |
| I/O read | `io_read_bytes` | bytes | `/proc/[pid]/io` → `read_bytes` |
| I/O write | `io_write_bytes` | bytes | `/proc/[pid]/io` → `write_bytes` |
| CPU % | `cpu_percent` | % | Delta `(utime+stime) / total_ticks × 100` |
| Start time | `start_time` | jiffies | `/proc/[pid]/stat` field 22 (starttime) |

---

### GPU Module (`gpu`)

#### Intel GPU

**Source files:** `/sys/bus/event_source/devices/i915/`, `/sys/devices/power/`, `perf_event_open()` syscall

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| Engine utilization | `engines[].busy_pct` | % | PMU counter `*-busy` via `perf_event_open` grouped reads |
| Engine name | `engines[].name` | string | Sysfs event filename (e.g., `rcs0`, `vcs0`, `bcs0`) |
| Actual frequency | `freq_act_mhz` | MHz | PMU counter `actual-frequency` |
| Requested frequency | `freq_req_mhz` | MHz | PMU counter `requested-frequency` (from `i915` PMU) |
| RC6 residency | `rc6_pct` | % | PMU counter `rc6-residency` |
| GPU power | `power_gpu_watts` | Watts | RAPL `energy-gpu` via `/sys/devices/power` PMU |
| Package power | `power_pkg_watts` | Watts | RAPL `energy-pkg` via `/sys/devices/power` PMU |
| IMC reads | `imc_reads_mbs` | MB/s | PMU counter `imc-reads` (when available) |
| IMC writes | `imc_writes_mbs` | MB/s | PMU counter `imc-writes` (when available) |

**Implementation details:**
- Uses **grouped perf events** (`PERF_FORMAT_GROUP | PERF_FORMAT_TOTAL_TIME_ENABLED`) for atomic multi-counter reads
- All engine/freq/rc6 counters in one group FD, RAPL counters in a separate group FD
- Config values dynamically parsed from sysfs event files (supports both `config=` and `event=` prefixes)
- Formula: `pmu_calc(d, t, s) = (cur - prev) / d / t * s` where:
  - Engines: `d=1e9, t=deltaSeconds, s=100` → percentage
  - Frequency: `d=1, t=deltaSeconds, s=1` → MHz
  - Power: `d=1, t=deltaSeconds, s=scale` → Watts
- **Requires `cap_perfmon` capability** (see Build & Run section)

#### NVIDIA GPU

**Dependency:** NVML library (`libnvidia-ml.so`) — requires NVIDIA drivers installed

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| GPU name | `name` | string | `nvmlDeviceGetName()` |
| GPU utilization | `utilization_gpu` | % | `nvmlDeviceGetUtilizationRates()` |
| Memory utilization | `utilization_mem` | % | Same |
| Temperature | `temp_c` | °C | `nvmlDeviceGetTemperature()` |
| Power draw | `power_watts` | Watts | `nvmlDeviceGetPowerUsage()` / 1000 |
| Power limit | `power_limit_watts` | Watts | `nvmlDeviceGetEnforcedPowerLimit()` / 1000 |
| Core clock | `clock_core_mhz` | MHz | `nvmlDeviceGetClockInfo(GRAPHICS)` |
| Memory clock | `clock_mem_mhz` | MHz | `nvmlDeviceGetClockInfo(MEM)` |
| VRAM total | `vram_total` | bytes | `nvmlDeviceGetMemoryInfo()` |
| VRAM used | `vram_used` | bytes | Same |
| VRAM free | `vram_free` | bytes | Same |
| PCIe TX | `pcie_tx_kbs` | KB/s | `nvmlDeviceGetPcieThroughput(TX)` |
| PCIe RX | `pcie_rx_kbs` | KB/s | `nvmlDeviceGetPcieThroughput(RX)` |
| Processes | `processes` | array | `nvmlDeviceGetComputeRunningProcesses()` |

> If no NVIDIA GPU or drivers are present, this section is silently omitted.

#### AMD GPU

**Source files:** `/sys/class/drm/card*/device/` (vendor `0x1002`)

| Field | JSON Key | Unit | Source |
|-------|----------|------|--------|
| GPU utilization | `utilization_gpu` | % | `gpu_busy_percent` |
| VRAM total | `vram_total` | bytes | `mem_info_vram_total` |
| VRAM used | `vram_used` | bytes | `mem_info_vram_used` |
| Temperature | `temp_c` | °C | `hwmon/temp1_input` ÷ 1000 |
| Power draw | `power_watts` | Watts | `hwmon/power1_average` ÷ 1e6 |
| Core clock | `clock_core_mhz` | MHz | `pp_dpm_sclk` (active state marked with `*`) |
| Memory clock | `clock_mem_mhz` | MHz | `pp_dpm_mclk` (active state marked with `*`) |

---

## Output Formats

### JSON (default)
```bash
./gtop get                    # Pretty-printed JSON
./gtop get --compact          # Single-line JSON
```

### Flat
```bash
./gtop get --format flat      # Dot-notation flattened keys
```
Output example:
```json
{
  "cpu.usage_percent": 5.68,
  "cpu.power_watts": 13.04,
  "memory.total": 16620814336,
  "memory.used": 9677864960
}
```

---

## Architecture

All collectors are stateless between calls except for delta-based metrics (CPU usage, GPU counters, process CPU %), which maintain internal state via package-level variables.

The TUI is built with [termdash](https://github.com/mum4k/termdash) and uses `tcell` as the terminal backend. The `tui/` package is completely decoupled from CLI mode — it reuses the same `collector/` package.
