package commands

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(requestCmd)
}

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
		options := requestOptions{
			Method: args[0],
			URL:    args[1],
		}

		options.WriteStateFile, _ = cmd.Flags().GetString("write-state")
		options.ReadStateFile, _ = cmd.Flags().GetString("read-state")
		options.Headers, _ = cmd.Flags().GetStringSlice("header")
		options.BodyData, _ = cmd.Flags().GetString("data")
		options.VarSymbol, _ = cmd.Flags().GetString("var")
		options.OutputHeaders, _ = cmd.Flags().GetBool("output-headers")

		return request(options)
	},
}

func invokeRequest(method, url string) error {
	return nil
}
