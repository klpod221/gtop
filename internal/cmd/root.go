package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	Run: func(cmd *cobra.Command, args []string) {
		// By default, launch the TUI if no subcommands are provided
		tuiCmd.Run(cmd, args)
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
	// Root flags if any global config goes here
}
