// Command dh-mcp runs the devhive-tunnel MCP server over stdio so AI agents
// (Claude Desktop, Cursor, opencode, …) can create/join hives, chat and share
// files with end-to-end encryption.
package main

import (
	"fmt"
	"os"

	"devhive-tunnel/pkg/conf"
	"devhive-tunnel/pkg/mcp"
)

func main() {
	srv := mcp.New(os.Stdin, os.Stdout, conf.Version)
	if err := srv.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "dh-mcp:", err)
		os.Exit(1)
	}
}
