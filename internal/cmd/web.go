package cmd

import (
	"fmt"
	"os"

	"gtop/internal/config"
	"gtop/internal/server"

	"github.com/spf13/cobra"
)

var (
	webPort       int
	webConfigPath string
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the gtop Web UI server",
	Long:  `Starts an embedded HTTP server that serves the Vue frontend and a WebSocket endpoint for real-time telemetry.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := webConfigPath
		if cfgPath == "" {
			var err error
			cfgPath, err = config.DefaultConfigPath()
			if err != nil {
				return err
			}
		}

		// Load port from config if not overridden by flag
		port := webPort
		if !cmd.Flags().Changed("port") {
			cfg, err := config.Load(cfgPath)
			if err == nil && cfg.Web.Port > 0 {
				port = cfg.Web.Port
			}
		}

		srv := server.NewServer(port, cfgPath)
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	webCmd.Flags().IntVarP(&webPort, "port", "p", 8080, "Port to run the Web server on")
	webCmd.Flags().StringVar(&webConfigPath, "config", "", "Path to config.json (default: ~/.config/gtop/config.json)")
	rootCmd.AddCommand(webCmd)
}
