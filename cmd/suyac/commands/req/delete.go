package req

import (
	"fmt"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/dekarrin/suyac/cmd/suyac/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(deleteCmd)
}

var deleteCmd = &cobra.Command{
	Use:   "delete REQ [-F project_file]",
	Short: "Delete a request template",
	Long:  "Delete an existing request template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		opts := deleteOptions{
			projFile: commonflags.ReqProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqDelete(reqName, opts)
	},
}

type deleteOptions struct {
	projFile string
}

func invokeReqDelete(name string, opts deleteOptions) error {
	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)

	if _, ok := p.Templates[name]; !ok {
		return fmt.Errorf("no request template %s", name)
	}

	delete(p.Templates, name)

	// save the project file
	return p.PersistToDisk(false)
}
