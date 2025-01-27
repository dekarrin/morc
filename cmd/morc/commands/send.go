package commands

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use: "send REQ",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"send REQ [-k] [-V VAR=VALUE]... [output-flags]",
	},
	Short: "Send a request defined in a template (REQ)",
	Long: "Send a request by building it from a request template (REQ) stored in the project. All variables are " +
		"filled prior to sending and the request is sent to the remote server. The response is then printed. Any data " +
		"captured from the response is automatically stored to their respective variables.",
	Args:    cobra.ExactArgs(1),
	GroupID: "sending",
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args sendArgs
		if err := parseSendArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		io.Quiet = flags.BQuiet

		return invokeSend(io, args.projFile, args.req, args.oneTimeVars, args.skipVerify, args.prefixOverride, args.outputCtrl)
	},
}

func init() {
	sendCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	sendCmd.PersistentFlags().StringArrayVarP(&flags.Vars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Overrides any value currently in the store. The argument to this flag must be in `VAR=VALUE` format.")
	sendCmd.PersistentFlags().BoolVarP(&flags.BInsecure, "insecure", "k", false, "Disable all verification of server certificates when sending requests over TLS (HTTPS)")
	sendCmd.PersistentFlags().StringVarP(&flags.VarPrefix, "var-prefix", "p", "", "Temporarily override the prefix used to identify variables in the request template for the current request only. Only variables in the request template that start with `PREFIX` will be interpreted as variables.")
	sendCmd.PersistentFlags().BoolVarP(&flags.BQuiet, "quiet", "q", false, "Suppress all unnecessary output.")

	addRequestOutputFlags(sendCmd)

	rootCmd.AddCommand(sendCmd)
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeSend(io cmdio.IO, projFile, reqName string, varOverrides map[string]string, skipVerify bool, prefixOverride optionalC[string], oc morc.OutputControl) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	oc.Writer = io.Out

	results, err := p.Send(reqName, varOverrides, skipVerify, prefixOverride.Or(""), cmdio.HTTPClient, oc)
	if err != nil {
		return err
	}

	return persistSendResults(p, len(results.Captures) > 0, len(results.Cookies) > 0)
}

type sendArgs struct {
	projFile       string
	req            string
	oneTimeVars    map[string]string
	outputCtrl     morc.OutputControl
	skipVerify     bool
	prefixOverride optionalC[string]
}

func parseSendArgs(cmd *cobra.Command, posArgs []string, args *sendArgs) error {
	// send is a single-action command, so we will only be gathering pos args
	// and flags.
	args.projFile = projPathFromFlagsOrFile(cmd)
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error
	args.outputCtrl, err = gatherRequestOutputFlags(cmd)
	if err != nil {
		return err
	}

	if len(flags.Vars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range flags.Vars {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("var #%d (%q) is not in format key=value", idx+1, v)
			}

			varName, err := morc.ParseVarName(strings.ToUpper(parts[0]))
			if err != nil {
				return fmt.Errorf("var #%d (%q): %w", idx+1, v, err)
			}
			oneTimeVars[varName] = parts[1]
		}
		args.oneTimeVars = oneTimeVars
	}

	if flags.BInsecure {
		args.skipVerify = true
	}

	if cmd.Flags().Lookup("var-prefix").Changed {
		args.prefixOverride = optionalC[string]{v: flags.VarPrefix, set: true}
	}

	args.req = posArgs[0]

	return nil
}

func persistSendResults(p morc.Project, varsWereSet, cookiesWereSet bool) error {
	// if any variable changes occurred, persist to disk
	if varsWereSet {
		err := writeProject(p, false)
		if err != nil {
			return fmt.Errorf("save project to disk: %w", err)
		}
	}

	// persist history to disk
	if p.Config.RecordHistory {
		err := writeHistory(p)
		if err != nil {
			return fmt.Errorf("save history to disk: %w", err)
		}
	}

	// persist cookies to disk, if any
	if p.Config.RecordSession && cookiesWereSet {
		err := writeSession(p)
		if err != nil {
			return fmt.Errorf("save session to disk: %w", err)
		}
	}

	return nil
}
