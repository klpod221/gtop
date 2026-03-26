package cmd

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gtop/internal/agent"
	"gtop/internal/config"
	"gtop/internal/server"

	"github.com/spf13/cobra"
)

var (
	rootConfigPath string
)

var rootCmd = &cobra.Command{
	Use:   "gtop",
	Short: "Go Based Linux System Monitor",
	Long: `   ██████╗ ████████╗ ██████╗ ██████╗
  ██╔════╝ ╚══██╔══╝██╔═══██╗██╔══██╗
  ██║  ███╗   ██║   ██║   ██║██████╔╝
  ██║   ██║   ██║   ██║   ██║██╔═══╝
  ╚██████╔╝   ██║   ╚██████╔╝██║
   ╚═════╝    ╚═╝    ╚═════╝ ╚═╝
  Linux System Monitor 		by klpod221

gtop is a high-performance system telemetry agent.
It collects CPU, memory, disk, network, and GPU metrics purely in Go.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// When run without subcommands (e.g. as a systemd service),
		// check config for enabled components.
		cfgPath := rootConfigPath
		if cfgPath == "" {
			var err error
			cfgPath, err = config.DefaultConfigPath()
			if err != nil {
				// Fallback to TUI if config path can't be resolved
				tuiCmd.Run(cmd, args)
				return nil
			}
		}

		cfg, loadErr := config.Load(cfgPath)
		if loadErr != nil || (!cfg.EnabledAgent && !cfg.EnabledWeb) {
			// If not running as daemon explicitly inside systemd (by having --config), 
			// fallback to TUI for interactive use.
			if rootConfigPath != "" {
				return fmt.Errorf("daemon mode error: config load failed or no modules enabled: %v", loadErr)
			}
			tuiCmd.Run(cmd, args)
			return nil
		}

		var wg sync.WaitGroup
		errCh := make(chan error, 2)

		if cfg.EnabledWeb {
			wg.Add(1)
			go func() {
				defer wg.Done()
				port := cfg.Web.Port
				if port == 0 {
					port = 8080
				}
				srv := server.NewServer(port, cfgPath)
				if err := srv.Start(); err != nil {
					log.Printf("Web server error: %v", err)
					errCh <- err
				}
			}()
		}

		if cfg.EnabledAgent {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := agent.Run(agent.RunOptions{ConfigPath: cfgPath}); err != nil {
					log.Printf("Agent error: %v", err)
					errCh <- err
				}
			}()
		}

		wg.Wait()
		select {
		case err := <-errCh:
			return err
		default:
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootConfigPath, "config", "", "Path to config.json")
}
