package commands

import (
	"fmt"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

func init() {
	execCmd.PersistentFlags().StringVarP(&flagProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
	execCmd.PersistentFlags().StringArrayVarP(&flagVars, "var", "V", []string{}, "Temporarily set a variable's value at the start of the flow. Format is name:value")

	setupRequestOutputFlags("suyac exec", execCmd)

	rootCmd.AddCommand(execCmd)
}

type execOptions struct {
	projFile    string
	oneTimeVars map[string]string
	outputCtrl  suyac.OutputControl
}

var execCmd = &cobra.Command{
	Use:     "exec FLOW [-F project_file] [output_control_flags] [-V var:value]...",
	Short:   "Execute a flow of requests",
	Long:    "Execute a sequence of requests defined in a flow stored in the project. Initial variables can be set with -V.",
	Args:    cobra.ExactArgs(1),
	GroupID: "sending",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := execFlagsToOptions()
		if err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeExec(args[0], opts)
	},
}

func execFlagsToOptions() (execOptions, error) {
	opts := execOptions{}

	opts.projFile = flagProjectFile
	if opts.projFile == "" {
		return opts, fmt.Errorf("project file is set to empty string")
	}

	var err error
	opts.outputCtrl, err = gatherRequestOutputFlags("suyac exec")
	if err != nil {
		return opts, err
	}

	// check vars
	if len(flagVars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range flagVars {
			parts := strings.SplitN(v, ":", 2)
			if len(parts) != 2 {
				return opts, fmt.Errorf("var #%d (%q) is not in format key:value", idx+1, v)
			}
			oneTimeVars[parts[0]] = parts[1]
		}
		opts.oneTimeVars = oneTimeVars
	}

	return opts, nil
}

// invokeExec receives the name of the flow to execute and the options to use.
func invokeExec(flowName string, opts execOptions) error {
	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	flowName = strings.ToLower(flowName)

	// check if the project even has a flow with that name
	flow, ok := p.Flows[flowName]
	if !ok {
		return fmt.Errorf("no flow named %s", flowName)
	}

	// now get all the templates and ensure they are valid
	var templates []suyac.RequestTemplate
	for i, step := range flow.Steps {
		tmpl, ok := p.Templates[strings.ToLower(step.Template)]
		if !ok {
			return fmt.Errorf("flow %s calls non-existent request template %q in step #%d", flowName, step.Template, i-1)
		}
		if !tmpl.Sendable() {
			return fmt.Errorf("flow %s calls incomplete request template %s in step #%d", flowName, step.Template, i-1)
		}

		templates = append(templates, tmpl)
	}

	for i, tmpl := range templates {
		err := sendTemplate(p, tmpl, opts.oneTimeVars, opts.outputCtrl)
		if err != nil {
			return fmt.Errorf("step #%d: %w", i, err)
		}
	}

	return nil
}
