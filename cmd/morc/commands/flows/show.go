package flows

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(showCmd)
}

var showCmd = &cobra.Command{
	Use:   "show FLOW [-F project_file]",
	Short: "Show steps in a flow",
	Long:  "Print out the sequence of request template executions that will be perfomed when the given flow is executed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flowName := args[0]
		if flowName == "" {
			return fmt.Errorf("flow name cannot be empty")
		}

		opts := showOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeFlowShow(flowName, opts)
	},
}

type showOptions struct {
	projFile string
}

func invokeFlowShow(name string, opts showOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	name = strings.ToLower(name)

	flow, ok := p.Flows[name]
	if !ok {
		return fmt.Errorf("no flow named %s exists", name)
	}

	if len(flow.Steps) == 0 {
		fmt.Println("(no steps in flow)")
	}

	for i, step := range flow.Steps {
		req, ok := p.Templates[step.Template]
		if !ok {
			return fmt.Errorf("step %d calls a non-existent template %s", i+1, step.Template)
		}

		meth := req.Method
		if meth == "" {
			meth = "???"
		}

		fmt.Printf("%d: %s (%s %s)\n", i+1, step.Template, meth, req.URL)
	}

	return nil
}
