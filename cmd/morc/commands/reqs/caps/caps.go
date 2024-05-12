package caps

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "caps REQ [-F project_file]",
	Short: "List variable captures in a request template",
	Long:  "Print a listing of all variable captures that will be attempted on responses to requests made from this template.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := listOptions{
			projectFile: commonflags.ProjectFile,
		}

		if opts.projectFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqCapsList(reqName, opts)
	},
}

type listOptions struct {
	projectFile string
}

func invokeReqCapsList(name string, opts listOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projectFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)

	req, ok := p.Templates[name]
	if !ok {
		return fmt.Errorf("no request template %s", name)
	}

	if len(req.Captures) == 0 {
		fmt.Println("(none)")
	} else {
		for _, cap := range req.Captures {
			fmt.Printf("%s\n", cap)
		}
	}

	return nil
}
