package collector

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NetInterface holds per-interface network metrics matching btop's Net::collect()
type NetInterface struct {
	Name      string `json:"name"`
	IPv4      string `json:"ipv4,omitempty"`
	IPv6      string `json:"ipv6,omitempty"`
	MAC       string `json:"mac,omitempty"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	Connected bool   `json:"connected"`
}

// CollectNetwork replicates btop's Net::collect()
// btop: uses getifaddrs() and reads /sys/class/net/{iface}/statistics/*
// btop does NOT filter loopback — it lists all interfaces
func CollectNetwork() []NetInterface {
	var interfaces []NetInterface

	paths, _ := filepath.Glob("/sys/class/net/*")
	for _, p := range paths {
		name := filepath.Base(p)
		iface := NetInterface{Name: name}

		// Check if interface is running (btop checks IFF_RUNNING)
		operstate, _ := os.ReadFile(filepath.Join(p, "operstate"))
		iface.Connected = strings.TrimSpace(string(operstate)) == "up"

		// MAC address (btop reads /sys/class/net/{iface}/address as fallback when no IP)
		macData, _ := os.ReadFile(filepath.Join(p, "address"))
		if len(macData) > 0 {
			iface.MAC = strings.TrimSpace(string(macData))
		}

		// RX/TX bytes from sysfs statistics
		rxData, _ := os.ReadFile(filepath.Join(p, "statistics", "rx_bytes"))
		if len(rxData) > 0 {
			iface.RxBytes, _ = strconv.ParseUint(strings.TrimSpace(string(rxData)), 10, 64)
		}

		txData, _ := os.ReadFile(filepath.Join(p, "statistics", "tx_bytes"))
		if len(txData) > 0 {
			iface.TxBytes, _ = strconv.ParseUint(strings.TrimSpace(string(txData)), 10, 64)
		}

		// IP via Go net package (equivalent to getifaddrs syscall)
		ni, err := net.InterfaceByName(name)
		if err == nil {
			addrs, err := ni.Addrs()
			if err == nil {
				for _, addr := range addrs {
					ipNet, ok := addr.(*net.IPNet)
					if !ok {
						continue
					}
					if ipNet.IP.To4() != nil && iface.IPv4 == "" {
						iface.IPv4 = ipNet.IP.String()
					} else if ipNet.IP.To4() == nil && iface.IPv6 == "" {
						iface.IPv6 = ipNet.IP.String()
					}
				}
			}
		}

		// btop: if no IPv4/IPv6 found, use MAC address as ipv4 field
		if iface.IPv4 == "" && iface.IPv6 == "" {
			iface.IPv4 = iface.MAC
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces
}
