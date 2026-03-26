package cmd

import (
	"fmt"
	"os"

	"gtop/internal/mcp"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the gtop Model Context Protocol (MCP) server",
	Long:  `Starts an MCP server accepting JSON-RPC 2.0 requests over stdin and responding to stdout.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := mcp.NewServer()
		if err := srv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "MCP Server error: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
