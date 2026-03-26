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
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run gtop as a background telemetry daemon",
	Long: `Runs gtop as a persistent background agent.

It reads configuration from ~/.config/gtop/config.json
(auto-created with defaults if missing), collects system metrics at the
configured interval, and POSTs them to the configured server endpoint.

Use 'gtop service install' to register it as a systemd service.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return agent.Run(agent.RunOptions{
			ConfigPath: agentConfigPath,
			DryRun:     agentDryRun,
			Once:       agentOnce,
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

func init() {
	// agent (root)
	agentCmd.PersistentFlags().StringVar(&agentConfigPath, "config", "", "Path to config.json (default: ~/.config/gtop/config.json)")
	agentCmd.Flags().BoolVar(&agentDryRun, "dry-run", false, "Collect metrics but print to stderr instead of sending")
	agentCmd.Flags().BoolVar(&agentOnce, "once", false, "Run one collection cycle then exit")

	// config --init
	agentConfigCmd.Flags().Bool("init", false, "Write default config if file does not exist, then print it")

	agentCmd.AddCommand(agentConfigCmd)

	rootCmd.AddCommand(agentCmd)
}
