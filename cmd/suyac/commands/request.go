package commands

import "github.com/spf13/cobra"

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Make an arbitrary HTTP request",
	Long:  "Creates a new request and sends it using the specified method. The method may be non-standard.",
}
