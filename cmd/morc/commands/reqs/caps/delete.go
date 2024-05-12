package caps

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(deleteCmd)
}

var deleteCmd = &cobra.Command{
	Use:   "delete REQ VAR [-F project_file]",
	Short: "Delete a variable capture from a request template",
	Long:  "Delete a variable capture from a request template",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		varName := args[1]
		if varName == "" {
			return fmt.Errorf("variable name cannot be empty")
		}

		opts := deleteOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqCapsDelete(reqName, varName, opts)
	},
}

type deleteOptions struct {
	projFile string
}

func invokeReqCapsDelete(name, varName string, opts deleteOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)
	req, ok := p.Templates[name]
	if !ok {
		return fmt.Errorf("no request template %s", name)
	}

	// var name normalized to upper case
	varUpper := strings.ToUpper(varName)
	if len(req.Captures) > 0 {
		if _, ok := req.Captures[varUpper]; !ok {
			return fmt.Errorf("no capture defined in %s for %s", name, varUpper)
		}
	}

	// remove the capture
	delete(req.Captures, varUpper)
	p.Templates[name] = req

	// save the project file
	return p.PersistToDisk(false)
}
