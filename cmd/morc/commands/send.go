package commands

import (
	"fmt"
	"sort"
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
		if err := parseSendArgs(posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		return invokeSend(io, args.projFile, args.req, args.oneTimeVars, args.skipVerify, args.outputCtrl)
	},
}

func init() {
	sendCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	sendCmd.PersistentFlags().StringArrayVarP(&flags.Vars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Overrides any value currently in the store. The argument to this flag must be in `VAR=VALUE` format.")
	sendCmd.PersistentFlags().BoolVarP(&flags.BInsecure, "insecure", "k", false, "Disable all verification of server certificates when sending requests over TLS (HTTPS)")

	addRequestOutputFlags(sendCmd)

	rootCmd.AddCommand(sendCmd)
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeSend(io cmdio.IO, projFile, reqName string, varOverrides map[string]string, skipVerify bool, oc morc.OutputControl) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)

	// check if the project already has a request with the same name
	tmpl, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	oc.Writer = io.Out
	_, err = sendTemplate(&p, tmpl, p.Vars.MergedSet(varOverrides), skipVerify, oc)
	return err
}

type sendArgs struct {
	projFile    string
	req         string
	oneTimeVars map[string]string
	outputCtrl  morc.OutputControl
	skipVerify  bool
}

func parseSendArgs(posArgs []string, args *sendArgs) error {
	// send is a single-action command, so we will only be gathering pos args
	// and flags.
	args.projFile = flags.ProjectFile
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error
	args.outputCtrl, err = gatherRequestOutputFlags()
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

	args.req = posArgs[0]

	return nil
}

func sendTemplate(p *morc.Project, tmpl morc.RequestTemplate, vars map[string]string, skipVerify bool, oc morc.OutputControl) (morc.SendResult, error) {
	// TODO: flows will call this and persist on EVERY request which is probably not needed.

	if tmpl.Method == "" {
		return morc.SendResult{}, fmt.Errorf("request template %s has no method set", tmpl.Name)
	}

	if tmpl.URL == "" {
		return morc.SendResult{}, fmt.Errorf("request template %s has no URL set", tmpl.Name)
	}

	varSymbol := "$"

	sendOpts := morc.SendOptions{
		Vars:               vars,
		Body:               tmpl.Body,
		Headers:            tmpl.Headers,
		Output:             oc,
		CookieLifetime:     p.Config.CookieLifetime,
		InsecureSkipVerify: skipVerify,
	}

	capVarNames := []string{}
	for k := range tmpl.Captures {
		capVarNames = append(capVarNames, k)
	}
	sort.Strings(capVarNames)
	for _, k := range capVarNames {
		sendOpts.Captures = append(sendOpts.Captures, tmpl.Captures[k])
	}

	if len(p.Session.Cookies) > 0 {
		sendOpts.Cookies = p.Session.Cookies
	}

	// inject the http client, in case we are to use a specific one
	sendOpts.Client = cmdio.HTTPClient

	result, err := morc.Send(tmpl.Method, tmpl.URL, varSymbol, sendOpts)
	if err != nil {
		return result, err
	}

	// if any variable changes occurred, persist to disk
	if len(result.Captures) > 0 {
		for k, v := range result.Captures {
			p.Vars.Set(k, v)
		}
		err := writeProject(*p, false)
		if err != nil {
			return result, fmt.Errorf("save project to disk: %w", err)
		}
	}

	// persist history
	if p.Config.RecordHistory {
		entry := morc.HistoryEntry{
			Template: tmpl.Name,
			ReqTime:  result.SendTime,
			RespTime: result.RecvTime,
			Request:  result.Request,
			Response: result.Response,
			Captures: result.Captures,
		}

		p.History = append(p.History, entry)
		err := writeHistory(*p)
		if err != nil {
			return result, fmt.Errorf("save history to disk: %w", err)
		}
	}

	// persist cookies
	if p.Config.RecordSession && len(result.Cookies) > 0 {
		p.Session.Cookies = result.Cookies

		err := writeSession(*p)
		if err != nil {
			return result, fmt.Errorf("save session to disk: %w", err)
		}
	}

	return result, nil
}
