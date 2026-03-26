package cmd

import (
	"fmt"
	"os"

	"gtop/internal/server"

	"github.com/spf13/cobra"
)

var webPort int

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the gtop Web UI server",
	Long:  `Starts an embedded HTTP server that serves the Vue frontend and a WebSocket endpoint for real-time telemetry.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := server.NewServer(webPort)
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	webCmd.Flags().IntVarP(&webPort, "port", "p", 8080, "Port to run the Web server on")
	rootCmd.AddCommand(webCmd)
}
