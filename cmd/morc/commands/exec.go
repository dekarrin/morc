package commands

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use: "exec FLOW",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"exec FLOW [-k] [-p PREFIX] [-V VAR=VALUE]... [output-flags]",
	},
	Short:   "Execute a flow of requests",
	Long:    "Execute a sequence of requests defined in a flow stored in the project. Initial variable values can be set with -V and will override any in the store before the first request in the flow is executed.",
	Args:    cobra.ExactArgs(1),
	GroupID: "sending",
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args execArgs
		if err := parseExecArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		return invokeExec(io, args.projFile, args.flow, args.oneTimeVars, args.skipVerify, args.prefixOverride, args.outputCtrl)
	},
}

func init() {
	execCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	execCmd.PersistentFlags().StringArrayVarP(&flags.Vars, "var", "V", []string{}, "Temporarily set a variable's value at the start of the flow. The argument to this flag must be in `VAR=VALUE` format.")
	execCmd.PersistentFlags().BoolVarP(&flags.BInsecure, "insecure", "k", false, "Disable all verification of server certificates when sending requests over TLS (HTTPS)")
	execCmd.PersistentFlags().StringVarP(&flags.VarPrefix, "var-prefix", "p", "", "Temporarily override the prefix used to identify variables in the request templates in the executed flow. Only variables in the request templates that start with `PREFIX` will be interpreted as variables.")

	addRequestOutputFlags(execCmd)

	rootCmd.AddCommand(execCmd)
}

// invokeExec receives the name of the flow to execute and the options to use.
func invokeExec(io cmdio.IO, projFile, flowName string, initialVarOverrides map[string]string, skipVerify bool, prefixOverride optionalC[string], oc morc.OutputControl) error {
	// load the project file
	p, err := readProject(projFile, true)
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
	for k, v := range initialVarOverrides {
		varOverrides[strings.ToUpper(k)] = v
	}

	varPrefix := prefixOverride.Or(p.VarPrefix())

	oc.Writer = io.Out
	for i, tmpl := range templates {
		// persistence should be covered in sendTemplate
		result, err := sendTemplate(&p, tmpl, p.Vars.MergedSet(varOverrides), skipVerify, varPrefix, oc)
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

type execArgs struct {
	projFile string

	flow           string
	oneTimeVars    map[string]string
	outputCtrl     morc.OutputControl
	skipVerify     bool
	prefixOverride optionalC[string]
}

func parseExecArgs(cmd *cobra.Command, posArgs []string, args *execArgs) error {
	args.projFile = projPathFromFlagsOrFile(cmd)
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	args.skipVerify = flags.BInsecure

	var err error
	args.outputCtrl, err = gatherRequestOutputFlags(cmd)
	if err != nil {
		return err
	}

	// check vars
	if len(flags.Vars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range flags.Vars {
			parts := strings.SplitN(v, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("var #%d (%q) is not in format key:value", idx+1, v)
			}
			oneTimeVars[parts[0]] = parts[1]
		}
		args.oneTimeVars = oneTimeVars
	}

	if cmd.Flags().Lookup("var-prefix").Changed {
		args.prefixOverride = optionalC[string]{v: flags.VarPrefix, set: true}
	}

	args.flow = posArgs[0]

	return nil
}
