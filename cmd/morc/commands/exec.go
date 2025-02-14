package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use: "exec EXECABLE",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"exec EXECABLE [-ak] [-p PREFIX] [-V VAR=VALUE]... [output-flags]",
	},
	Short: "Execute a flow of requests or an auth method",
	Long: "Execute a sequence of requests defined in a flow stored in the project. Initial variable values can be set " +
		"with -V and will override any in the store before the first request is executed.\n" +
		"\n" +
		"If -A is given, EXECABLE is interpreted as the name of a dynamic auth method rather than a flow. The auth " +
		"method will be checked for a currently valid auth proof, and if one is not present, it will execute its " +
		"configured request sequence to obtain one using the provided output flags (-p and -V options are ignored " +
		"for auth sending) and will save it to its cache. After execution, the resulting proof is displayed.",
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
		io.Quiet = flags.BQuiet

		return invokeExec(io, args.projFile, args.execable, args.isAuth, args.oneTimeVars, args.skipVerify, args.prefixOverride, args.outputCtrl)
	},
}

func init() {
	execCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	execCmd.PersistentFlags().StringArrayVarP(&flags.Vars, "var", "V", []string{}, "Temporarily set a variable's value at the start of the flow. The argument to this flag must be in `VAR=VALUE` format.")
	execCmd.PersistentFlags().BoolVarP(&flags.BInsecure, "insecure", "k", false, "Disable all verification of server certificates when sending requests over TLS (HTTPS)")
	execCmd.PersistentFlags().StringVarP(&flags.VarPrefix, "var-prefix", "p", "", "Temporarily override the prefix used to identify variables in the request templates in the executed flow. Only variables in the request templates that start with `PREFIX` will be interpreted as variables.")
	execCmd.PersistentFlags().BoolVarP(&flags.BQuiet, "quiet", "q", false, "Suppress all unnecessary output.")
	execCmd.PersistentFlags().BoolVarP(&flags.BAuth, "auth", "a", false, "Interpret EXECABLE as the name of an auth method instead of a flow, and print the final obtained proof.")

	addRequestOutputFlags(execCmd)

	rootCmd.AddCommand(execCmd)
}

// invokeExec receives the name of the execable and the options to use.
func invokeExec(io cmdio.IO, projFile, execName string, isAuth bool, initialVarOverrides map[string]string, skipVerify bool, prefixOverride optionalC[string], oc morc.OutputControl) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	oc.Writer = io.Out

	var results []morc.SendResult
	var updatedAuths []string
	if isAuth {
		execLower := strings.ToLower(execName)
		auth, ok := p.Auths[execLower]
		if !ok {
			return morc.NewAuthNotFoundError(execName)
		}

		var ap morc.AuthProof
		ap, results, updatedAuths, err = p.ExecAuth(&auth, skipVerify, cmdio.HTTPClient, oc)
		if err != nil {
			return err
		}

		p.Auths[execLower] = auth

		io.PrintLoudf("Got auth after ")
		io.Printf("%s\n", io.CountOf(len(results), "request"))
		io.PrintLoudf("Auth proof: ")
		io.Printf("%s\n", ap.Secret())

		exp := ap.Expiration()
		if exp.IsZero() {
			io.Printf("(no expiration)\n")
		} else {
			io.PrintLoudf("Expires: ")
			io.Printf("%s\n", exp.Format(time.RFC3339))
		}
	} else {
		var err error
		results, updatedAuths, err = p.Exec(execName, initialVarOverrides, skipVerify, prefixOverride.Or(""), cmdio.HTTPClient, oc)
		if err != nil {
			return err
		}
	}

	var varsSet, cookiesSet bool
	for _, r := range results {
		if len(r.Captures) > 0 {
			varsSet = true
			break
		}
	}
	for _, r := range results {
		if len(r.Cookies) > 0 {
			cookiesSet = true
			break
		}
	}

	return persistSendResults(p, varsSet, cookiesSet, len(updatedAuths) > 0)
}

type execArgs struct {
	projFile string

	execable       string
	isAuth         bool
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

	args.execable = posArgs[0]

	if flags.BAuth {
		args.isAuth = true
	}

	return nil
}
