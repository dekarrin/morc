package flows

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
	Use:   "delete FLOW [-F project_file]",
	Short: "Delete a flow",
	Long:  "Delete an existing flow from the project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flowName := args[0]
		if flowName == "" {
			return fmt.Errorf("flow name cannot be empty")
		}

		opts := deleteOptions{
			projFile: commonflags.ReqProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeFlowDelete(flowName, opts)
	},
}

type deleteOptions struct {
	projFile string
}

func invokeFlowDelete(name string, opts deleteOptions) error {
	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	name = strings.ToLower(name)

	if _, ok := p.Flows[name]; !ok {
		return fmt.Errorf("no flow named %s", name)
	}

	delete(p.Flows, name)

	// save the project file
	return p.PersistToDisk(false)
}
