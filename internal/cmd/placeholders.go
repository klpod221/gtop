package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run gtop as a background telemetry daemon",
	Long:  `Runs gtop as a background agent, exposing system metrics via HTTP/gRPC.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Agent mode is not implemented yet. Placeholder ready!")
	},
}

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Launch the gtop Web UI server",
	Long:  `Starts a web server hosting the gtop visual dashboard via HTTP/WebSockets.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Web UI mode is not implemented yet. Placeholder ready!")
	},
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the gtop Model Context Protocol (MCP) server",
	Long:  `Starts an MCP server allowing LLMs to read real-time system metrics directly.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("MCP Server mode is not implemented yet. Placeholder ready!")
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(webCmd)
	rootCmd.AddCommand(mcpCmd)
}
