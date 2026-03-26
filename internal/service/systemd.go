package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// ServiceType controls which component(s) the unit runs.
type ServiceType struct {
	Agent bool
	Web   bool
}

const systemdUnitTemplate = `[Unit]
Description=gtop System Monitor
Documentation=https://github.com/klpod221/gtop
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExecPath}} --config '{{.ConfigPath}}'
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gtop

[Install]
WantedBy=multi-user.target
`

const userSystemdUnitTemplate = `[Unit]
Description=gtop System Monitor
Documentation=https://github.com/klpod221/gtop
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExecPath}} --config '{{.ConfigPath}}'
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=default.target
`

const serviceName = "gtop"

// ServiceInstallOptions controls how the systemd unit is installed.
type ServiceInstallOptions struct {
	// UserMode installs the unit as a user service (~/.config/systemd/user/)
	// instead of a system-wide service (/etc/systemd/system/).
	UserMode bool
	// ConfigPath is the absolute path to the configuration file to use for the service.
	ConfigPath string
}

// Install creates and enables the systemd unit.
func Install(opts ServiceInstallOptions) error {
	execPath, err := resolveExecPath()
	if err != nil {
		return err
	}

	unitContent, err := renderUnit(execPath, opts.ConfigPath, opts.UserMode)
	if err != nil {
		return fmt.Errorf("rendering unit template: %w", err)
	}

	unitPath, err := unitFilePath(opts.UserMode)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		return fmt.Errorf("creating systemd unit directory: %w", err)
	}
	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("writing unit file to %s: %w", unitPath, err)
	}
	fmt.Printf("Unit file written to: %s\n", unitPath)

	return enableAndStart(opts.UserMode)
}

// Uninstall stops, disables, and removes the gtop systemd unit.
func Uninstall(opts ServiceInstallOptions) error {
	unitPath, err := unitFilePath(opts.UserMode)
	if err != nil {
		return err
	}

	_ = runSystemctl(opts.UserMode, "stop", serviceName)
	_ = runSystemctl(opts.UserMode, "disable", serviceName)

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file %s: %w", unitPath, err)
	}
	fmt.Printf("Removed unit file: %s\n", unitPath)

	return runSystemctl(opts.UserMode, "daemon-reload")
}

// Status checks whether the gtop service is active via systemctl.
func Status(opts ServiceInstallOptions) error {
	return runSystemctl(opts.UserMode, "status", serviceName)
}

func enableAndStart(userMode bool) error {
	if err := runSystemctl(userMode, "daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctl(userMode, "enable", "--now", serviceName); err != nil {
		return err
	}
	fmt.Println("gtop service enabled and started.")
	return nil
}

func runSystemctl(userMode bool, args ...string) error {
	baseArgs := args
	if userMode {
		baseArgs = append([]string{"--user"}, args...)
	}
	cmd := exec.Command("systemctl", baseArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func unitFilePath(userMode bool) (string, error) {
	if userMode {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home dir: %w", err)
		}
		return filepath.Join(home, ".config", "systemd", "user", serviceName+".service"), nil
	}
	return "/etc/systemd/system/" + serviceName + ".service", nil
}

func resolveExecPath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve executable path: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot get absolute executable path: %w", err)
	}
	return abs, nil
}

func renderUnit(execPath, configPath string, userMode bool) (string, error) {
	tmplSrc := systemdUnitTemplate
	if userMode {
		tmplSrc = userSystemdUnitTemplate
	}
	tmpl, err := template.New("unit").Parse(tmplSrc)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	
	data := struct {
		ExecPath   string
		ConfigPath string
	}{
		ExecPath:   execPath,
		ConfigPath: configPath,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
