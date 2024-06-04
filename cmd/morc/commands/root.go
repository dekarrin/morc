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
		"QUICKSTART:\n\n" +
		"Create a new project with `morc init`. Create a request in the project with `morc reqs --new`. Send the " +
		"request with `morc send`\n\n" +
		"OR\n\n" +
		"Send a one-off request without using a MORC project with `morc oneoff` (or one of the quick-method commands: " +
		"`morc get`, `morc post`, `morc head`, etc).\n\n" +
		"PROJECT USAGE:\n\n" +
		"First, a project is created with either `morc init` or `morc proj --new`. Requests are defined in a project and " +
		"manipulated using `morc reqs`. Once a request is created, it exists in the project until deleted, and can be " +
		"sent using `morc send`. Requests can be collected into sequences of requests (flows) using `morc flows`. When " +
		"a flow is executed with `morc exec`, each request in it is sent, one after each other.\n\n" +
		"A project is stored on disk as one or more files; a main project file (called just project file for short), " +
		"and possibly a file for tracking request history and a file for tracking set cookies. All files beside the " +
		"project file itself are referred to by path within the project file; MORC only needs the location of that file " +
		"when executing. When a project is created using `morc init`, and by default when one is created with `morc " +
		"proj --new`, the project file is created at path .morc/project.json relative to the working directory the " +
		"command is executed from. This is also where MORC looks for a project file by default whenever executing " +
		"project commands. If a different project file is to be pointed to, all project commands accept a " +
		"--project_file/-F flag that allows it to be specified." +
		"A MORC project records request history, tracks cookes set by responses, and holds a variable store. The " +
		"variable store is further split into user-defined environments; by default, variables are defined and stored " +
		"in a default environment. Variables are used to fill request templates at send time; any variable in request " +
		"headers, body, or URL in form '${VAR_NAME}' is replaced with the value of the variable called VAR_NAME. By " +
		"default, this is attempted to be taken from the current variable store environment, but can be overriden " +
		"temporarily with flags at sendtime." +
		"",
	Version:       morc.Version,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
