package cmd

import (
	"fmt"
	"os"

	"gtop/internal/agent"
	"gtop/internal/config"

	"github.com/spf13/cobra"
)

var (
	agentConfigPath string
	agentDryRun     bool
	agentOnce       bool
	agentUserMode   bool
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run gtop as a background telemetry daemon",
	Long: `Runs gtop as a persistent background agent.

It reads configuration from ~/.config/oikos-agent/config.json
(auto-created with defaults if missing), collects system metrics at the
configured interval, and POSTs them to the configured server endpoint.

Use 'gtop agent install' to register it as a systemd service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return agent.Run(agent.RunOptions{
			ConfigPath: agentConfigPath,
			DryRun:     agentDryRun,
			Once:       agentOnce,
		})
	},
}

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gtop-agent as a systemd service",
	Long: `Writes a systemd unit file and enables the gtop-agent service.

By default, installs as a system-wide service (/etc/systemd/system/)
which requires root. Use --user to install as a user service instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return agent.Install(agent.ServiceInstallOptions{
			UserMode: agentUserMode,
		})
	},
}

var agentUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the gtop-agent systemd service",
	Long:  `Stops, disables, and removes the gtop-agent systemd unit file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return agent.Uninstall(agent.ServiceInstallOptions{
			UserMode: agentUserMode,
		})
	},
}

var agentConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Print or initialise the agent config file",
	Long: `Prints the current config file path and its contents.

If --init is passed (or the file does not exist), a default config
is written to the path and then printed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := agentConfigPath
		if path == "" {
			var err error
			path, err = config.DefaultConfigPath()
			if err != nil {
				return err
			}
		}

		cfg, err := config.Load(path) // creates default if missing
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Config file: %s\n\n", path)

		initFlag, _ := cmd.Flags().GetBool("init")
		if initFlag {
			if writeErr := config.Write(path, cfg); writeErr != nil {
				return fmt.Errorf("writing config: %w", writeErr)
			}
			fmt.Fprintf(os.Stderr, "Default config written.\n\n")
		}

		data, _ := os.ReadFile(path)
		fmt.Println(string(data))
		return nil
	},
}

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check whether the agent daemon is running",
	Long:  `Reads the PID file defined in config and checks if the process is alive.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := agentConfigPath
		if path == "" {
			var err error
			path, err = config.DefaultConfigPath()
			if err != nil {
				return err
			}
		}

		cfg, err := config.Load(path)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		pidFile := cfg.Agent.PIDFile
		if pidFile == "" {
			fmt.Println("No PID file configured.")
			return nil
		}

		data, err := os.ReadFile(pidFile)
		if os.IsNotExist(err) {
			fmt.Println("gtop-agent is NOT running (PID file not found).")
			return nil
		} else if err != nil {
			return fmt.Errorf("reading PID file %s: %w", pidFile, err)
		}

		pidStr := string(data)
		// Check if the process is alive by sending signal 0.
		var pid int
		if _, scanErr := fmt.Sscanf(pidStr, "%d", &pid); scanErr != nil {
			return fmt.Errorf("invalid PID in %s: %s", pidFile, pidStr)
		}

		proc, findErr := os.FindProcess(pid)
		if findErr != nil {
			fmt.Printf("gtop-agent is NOT running (PID %d not found).\n", pid)
			return nil
		}

		// On Linux, FindProcess always succeeds; signal 0 confirms liveness.
		if sigErr := proc.Signal(os.Signal(nil)); sigErr != nil {
			fmt.Printf("gtop-agent is NOT running (PID %d, signal check failed).\n", pid)
		} else {
			fmt.Printf("gtop-agent is RUNNING (PID %d).\n", pid)
		}
		return nil
	},
}

func init() {
	// agent (root)
	agentCmd.PersistentFlags().StringVar(&agentConfigPath, "config", "", "Path to config.json (default: ~/.config/oikos-agent/config.json)")
	agentCmd.Flags().BoolVar(&agentDryRun, "dry-run", false, "Collect metrics but print to stderr instead of sending")
	agentCmd.Flags().BoolVar(&agentOnce, "once", false, "Run one collection cycle then exit")

	// install / uninstall share --user flag
	agentInstallCmd.Flags().BoolVar(&agentUserMode, "user", false, "Install as a user service (~/.config/systemd/user/) instead of system-wide")
	agentUninstallCmd.Flags().BoolVar(&agentUserMode, "user", false, "Uninstall user service instead of system-wide service")

	// config --init
	agentConfigCmd.Flags().Bool("init", false, "Write default config if file does not exist, then print it")

	agentCmd.AddCommand(agentInstallCmd)
	agentCmd.AddCommand(agentUninstallCmd)
	agentCmd.AddCommand(agentConfigCmd)
	agentCmd.AddCommand(agentStatusCmd)

	rootCmd.AddCommand(agentCmd)
}
