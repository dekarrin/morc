package caps

import (
	"fmt"

	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new REQ VAR CAP [-F project_file]",
	Short: "Add a new variable capture to a request template",
	Long:  "Add a new variable capture to a request template. The capture will be attempted on responses to requests made from this template. VAR must be a variable name containing only letters, numbers, or underscore. CAP must be either ':START,END' for a byte offset (ex: \":4,20\") or a jq-ish path with only keys and variable indexes (ex: \"records[1].auth.token\")",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		varName := args[1]
		if varName == "" {
			return fmt.Errorf("variable name cannot be empty")
		}

		varCap := args[2]
		if varCap == "" {
			return fmt.Errorf("variable capture spec cannot be empty")
		}

		opts := newOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return nil
	},
}

type newOptions struct {
	projFile string
}
