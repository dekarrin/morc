package commands

import (
	"fmt"
	"os"

	"github.com/dekarrin/suyac/cmd/suyac/commands/proj"
	"github.com/dekarrin/suyac/cmd/suyac/commands/req"
	"github.com/spf13/cobra"
)

var (
	projMetaCommands = &cobra.Group{
		Title: "Project Commands",
		ID:    "project",
	}
	sendingCommands = &cobra.Group{
		Title: "Request Sending",
		ID:    "sending",
	}
	quickreqCommands = &cobra.Group{
		Title: "Request Quick Commands",
		ID:    "quickreqs",
	}
)

func init() {
	rootCmd.AddGroup(projMetaCommands)
	rootCmd.AddGroup(sendingCommands)
	rootCmd.AddGroup(quickreqCommands)
	rootCmd.AddCommand(proj.RootCmd)
	rootCmd.AddCommand(req.RootCmd)
}

var rootCmd = &cobra.Command{
	Use:   "suyac",
	Short: "Suyac is a scriptable CLI REST client",
	Long:  "A CLI REST client that allows you to script HTTP requests and responses",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
