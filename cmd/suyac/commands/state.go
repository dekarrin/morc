package commands

import "github.com/spf13/cobra"

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Read state data",
	Long:  "Load a file containing state data into memory and print out what it contains in human readable format.",
}
