package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
)

var (
	flagProjectFile string
)

func init() {
	sendCmd.PersistentFlags().StringVarP(&flagProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	sendCmd.PersistentFlags().StringArrayVarP(&flagVars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Format is name:value")

	setupRequestOutputFlags("morc send", sendCmd)

	rootCmd.AddCommand(sendCmd)
}

type sendOptions struct {
	projFile    string
	oneTimeVars map[string]string
	outputCtrl  morc.OutputControl
}

var sendCmd = &cobra.Command{
	Use:     "send REQ [-F project_file]",
	Short:   "Send a request defined in a template (req)",
	Long:    "Send a request by building it from a request template (req) stored in the project.",
	Args:    cobra.ExactArgs(1),
	GroupID: "sending",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := sendFlagsToOptions()
		if err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeSend(args[0], opts)
	},
}

func sendFlagsToOptions() (sendOptions, error) {
	opts := sendOptions{}

	opts.projFile = flagProjectFile
	if opts.projFile == "" {
		return opts, fmt.Errorf("project file is set to empty string")
	}

	var err error
	opts.outputCtrl, err = gatherRequestOutputFlags("morc send")
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

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeSend(reqName string, opts sendOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
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

	return sendTemplate(p, tmpl, opts.oneTimeVars, opts.outputCtrl)
}

func sendTemplate(p morc.Project, tmpl morc.RequestTemplate, vars map[string]string, oc morc.OutputControl) error {

	if tmpl.Method == "" {
		return fmt.Errorf("request template %s has no method set", tmpl.Name)
	}

	if tmpl.URL == "" {
		return fmt.Errorf("request template %s has no URL set", tmpl.Name)
	}

	varSymbol := "$"

	sendOpts := morc.SendOptions{
		Vars:           vars,
		Body:           tmpl.Body,
		Headers:        tmpl.Headers,
		Output:         oc,
		CookieLifetime: p.Config.CookieLifetime,
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

	result, err := morc.Send(tmpl.Method, tmpl.URL, varSymbol, sendOpts)
	if err != nil {
		return err
	}

	// if any variable changes occurred, persist to disk
	if len(result.Captures) > 0 {
		for k, v := range result.Captures {
			p.Vars.Set(k, v)
		}
		err := p.PersistToDisk(false)
		if err != nil {
			return fmt.Errorf("save project to disk: %w", err)
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
		err := p.PersistHistoryToDisk()
		if err != nil {
			return fmt.Errorf("save history to disk: %w", err)
		}
	}

	// persist cookies
	if p.Config.RecordSession && len(result.Cookies) > 0 {
		p.Session.Cookies = result.Cookies

		err := p.PersistSessionToDisk()
		if err != nil {
			return fmt.Errorf("save session to disk: %w", err)
		}
	}

	return nil
}
