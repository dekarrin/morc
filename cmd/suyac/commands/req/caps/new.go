package caps

import (
	"fmt"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/dekarrin/suyac/cmd/suyac/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new REQ VAR CAP [-F project_file]",
	Short: "Add a new variable capture to a request template",
	Long:  "Add a new variable capture to a request template. The capture will be attempted on responses to requests made from this template. VAR must be a variable name containing only letters, numbers, or underscore. CAP must be either ':START,END' for a byte offset (ex: \":4,20\") or a jq-ish path with only keys and variable indexes (ex: \"records[1].auth.token\")",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		opts := newOptions{
			projFile: commonflags.ReqProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		return invokeReqCapsNew(reqName, opts)
	},
}

type newOptions struct {
	projFile string
}

func invokeReqCapsNew(name string, opts newOptions) error {
	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)

	req, ok := p.Templates[name]
	if !ok {
		return fmt.Errorf("no request template %s", name)
	}

	// save the project file
	return p.PersistToDisk(false)
}
