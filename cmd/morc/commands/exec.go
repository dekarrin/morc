package commands

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cliflags"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

func init() {
	execCmd.PersistentFlags().StringVarP(&cliflags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	execCmd.PersistentFlags().StringArrayVarP(&cliflags.Vars, "var", "V", []string{}, "Temporarily set a variable's value at the start of the flow. The argument to this flag must be in `VAR=VALUE` format.")
	execCmd.PersistentFlags().BoolVarP(&cliflags.BInsecure, "insecure", "k", false, "Disable all verification of server certificates when sending requests over TLS (HTTPS)")

	setupRequestOutputFlags("morc exec", execCmd)

	rootCmd.AddCommand(execCmd)
}

type execOptions struct {
	projFile    string
	oneTimeVars map[string]string
	outputCtrl  morc.OutputControl
	skipVerify  bool
}

var execCmd = &cobra.Command{
	Use: "exec FLOW",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"exec FLOW [-k] [-V VAR=VALUE]... [output-flags]",
	},
	Short:   "Execute a flow of requests",
	Long:    "Execute a sequence of requests defined in a flow stored in the project. Initial variable values can be set with -V and will override any in the store before the first request in the flow is executed.",
	Args:    cobra.ExactArgs(1),
	GroupID: "sending",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := execFlagsToOptions()
		if err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		return invokeExec(io, args[0], opts)
	},
}

func execFlagsToOptions() (execOptions, error) {
	opts := execOptions{
		skipVerify: cliflags.BInsecure,
	}

	opts.projFile = cliflags.ProjectFile
	if opts.projFile == "" {
		return opts, fmt.Errorf("project file is set to empty string")
	}

	var err error
	opts.outputCtrl, err = gatherRequestOutputFlags("morc exec")
	if err != nil {
		return opts, err
	}

	// check vars
	if len(cliflags.Vars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range cliflags.Vars {
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
func invokeExec(io cmdio.IO, flowName string, opts execOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
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
	var templates []morc.RequestTemplate
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

	varOverrides := make(map[string]string)
	// copy in the one-time vars
	for k, v := range opts.oneTimeVars {
		varOverrides[strings.ToUpper(k)] = v
	}

	opts.outputCtrl.Writer = io.Out
	for i, tmpl := range templates {
		result, err := sendTemplate(&p, tmpl, p.Vars.MergedSet(varOverrides), opts.skipVerify, opts.outputCtrl)
		if err != nil {
			return fmt.Errorf("step #%d: %w", i, err)
		}

		// okay, need to update the varOverrides because if any were just
		// captured, THAT is the new canonical value of the var
		for k := range result.Captures {
			delete(varOverrides, strings.ToUpper(k))
		}
	}

	return nil
}
