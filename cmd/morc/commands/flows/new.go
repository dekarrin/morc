package flows

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

// TODO: swap all project file references to -P.
func init() {
	RootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new [-F project_file] NAME REQ1 REQ2 [REQ3...]",
	Short: "Create a new flow",
	Long:  "Create a new flow made up of one or more request template sends. The flow can later be executed by calling 'morc exec NAME'",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if name == "" {
			return fmt.Errorf("flow name cannot be empty")
		}

		// gather the request names
		reqNames := args[1:]
		for _, reqName := range reqNames {
			if reqName == "" {
				return fmt.Errorf("request name cannot be empty")
			}
		}

		opts := newOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeFlowsNew(name, reqNames, opts)
	},
}

type newOptions struct {
	projFile string
}

func invokeFlowsNew(name string, templates []string, opts newOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	name = strings.ToLower(name)

	// check if the project already has a flow with the same name
	if _, exists := p.Flows[name]; exists {
		return fmt.Errorf("flow %s already exists in project", name)
	}

	// check that each of the templates exist and create the flow steps
	var steps []morc.FlowStep
	for _, reqName := range templates {
		reqLower := strings.ToLower(reqName)
		if _, exists := p.Templates[reqLower]; !exists {
			return fmt.Errorf("no request template %q in project", reqName)
		}
		steps = append(steps, morc.FlowStep{
			Template: reqLower,
		})
	}

	// create the new flow
	flow := morc.Flow{
		Name:  name,
		Steps: steps,
	}

	p.Flows[name] = flow

	// save the project file
	return p.PersistToDisk(false)
}
