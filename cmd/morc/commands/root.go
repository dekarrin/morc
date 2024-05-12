package commands

import (
	"fmt"
	"os"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commands/flows"
	"github.com/dekarrin/morc/cmd/morc/commands/proj"
	"github.com/dekarrin/morc/cmd/morc/commands/reqs"
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
	rootCmd.AddCommand(reqs.RootCmd)
	rootCmd.AddCommand(flows.RootCmd)
}

var rootCmd = &cobra.Command{
	Use:           "morc",
	Short:         "Morc is a scriptable CLI REST client",
	Long:          "A CLI REST client that allows you to script HTTP requests and responses",
	Version:       morc.Version,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}