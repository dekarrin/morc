package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	flagWriteStateFile string
	flagReadStateFile  string
	flagHeaders        []string
	flagBodyData       string
	flagVarSymbol      string
	flagOutputHeaders  bool
)

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Make an arbitrary HTTP request",
	Long:  "Creates a new request and sends it using the specified method. The method may be non-standard.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// check flags and populate a requestOptions struct
		opts := requestOptions{
			stateFileOut: flagWriteStateFile,
			stateFileIn:  flagReadStateFile,
		}

		return invokeRequest(args[0], args[1])
	},
}

func init() {
	requestCmd.PersistentFlags().StringVarP(&flagWriteStateFile, "write-state", "b", "", "Write collected cookies and captured vars to the given file")
	requestCmd.PersistentFlags().StringVarP(&flagReadStateFile, "read-state", "c", "", "Read and use the cookies and vars from the given file")
	requestCmd.PersistentFlags().StringArrayVarP(&flagHeaders, "header", "H", []string{}, "Add a header to the request")
	requestCmd.PersistentFlags().StringVarP(&flagBodyData, "data", "d", "", "Add the given data as a body to the request; prefix with @ to read data from a file")
	requestCmd.PersistentFlags().StringVarP(&flagVarSymbol, "var-symbol", "v", "$", "The symbol to use for variable substitution")
	requestCmd.PersistentFlags().BoolVarP(&flagOutputHeaders, "output-headers", "o", false, "Output the headers of the response")

	rootCmd.AddCommand(requestCmd)
}

type requestOptions struct {
	stateFileOut  string
	stateFileIn   string
	headers       []string
	bodyData      string
	outputHeaders bool
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeRequest(method, url, varSymbol string, opts requestOptions) error {
	if opts.varSymbol == "" {
		return fmt.Errorf("variable symbol cannot be empty")
	}
}
