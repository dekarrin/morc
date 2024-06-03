package commands

import (
	"fmt"
	"os"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
)

var (
	projMetaCommands = &cobra.Group{
		Title: "Project Commands",
		ID:    "project",
	}
	sendingCommands = &cobra.Group{
		Title: "Request Sending Commands",
		ID:    "sending",
	}
	quickreqCommands = &cobra.Group{
		Title: "Oneoff Quick-Commands",
		ID:    "quickreqs",
	}
)

func init() {
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(helpTemplate)

	rootCmd.AddGroup(projMetaCommands)
	rootCmd.AddGroup(sendingCommands)
	rootCmd.AddGroup(quickreqCommands)
}

var rootCmd = &cobra.Command{
	Use:   "morc COMMAND",
	Short: "MORC is a scriptable CLI REST client",
	Long: "MORC, the MORonically-simple Client, is a CLI REST client that allows you to script HTTP requests " +
		"and responses. It's main focus is project-oriented use, in which requests templates are defined in a " +
		"MORC project and are executed at a later time. It also supports one-off requests to quickly send a test " +
		"request, or if slightly more functionality is needed, limited state information may be optionally saved in " +
		"one-off mode.\n\n" +
		"Quickstart:\n\n" +
		"Create a new project with `morc init`. Create a request in the project with `morc reqs --new`. Send the " +
		"request with `morc send`\n\n" +
		"OR\n\n" +
		"Send a one-off request without using a MORC project with `morc oneoff` (or one of the quick-method commands: " +
		"`morc get`, `morc post`, `morc head`, etc).",
	Version:       morc.Version,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
