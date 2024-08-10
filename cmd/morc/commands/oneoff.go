package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var oneoffCmd = &cobra.Command{
	Use: "oneoff METHOD URL",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"oneoff METHOD URL [-HdCVkbcp] [output-flags]",
	},
	GroupID: "sending",
	Short:   "Make an arbitrary one-off HTTP request",
	Long: "Creates a new request and sends it using the specified method. The method may be non-standard. No " +
		"project file is consulted, but state files may be read and written.",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args oneoffArgs
		if err := parseOneoffArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		io.Quiet = flags.BQuiet

		return makeOneoffRequest(io, args)
	},
}

func init() {
	addOneoffRequestFlags(oneoffCmd)
	rootCmd.AddCommand(oneoffCmd)

	quickMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE"}
	for _, meth := range quickMethods {
		addQuickMethodCommand(meth)
	}
}

func addOneoffRequestFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&flags.WriteStateFile, "write-state", "b", "", "Write collected cookies and captured vars to statefile `FILE`.")
	cmd.PersistentFlags().StringVarP(&flags.ReadStateFile, "read-state", "c", "", "Read and use the cookies and vars saved in statefile `FILE`.")
	cmd.PersistentFlags().StringArrayVarP(&flags.Headers, "header", "H", []string{}, "Add a header to the request. Argument is in form `KEY:VALUE` (spaces after the colon are allowed). May be set multiple times.")
	cmd.PersistentFlags().StringVarP(&flags.BodyData, "data", "d", "", "Add the given `DATA` as a body to the request; prefix with '@' to instead interperet DATA as a filename that body data is to be read from.")
	cmd.PersistentFlags().StringVarP(&flags.VarPrefix, "var-prefix", "p", "$", "Set the leading variable symbol used to indicate the start of a variable in the request to `PREFIX`.")
	cmd.PersistentFlags().StringArrayVarP(&flags.CaptureVars, "capture-var", "C", []string{}, "Get a variable's value from the response. Argument is in format `VAR:SPEC`. The SPEC part has format ':START,END' for byte offset (note the leading colon, resulting in 'VAR::START,END'), or 'path[0].to.value' (jq-ish syntax) for JSON body data.")
	cmd.PersistentFlags().StringArrayVarP(&flags.Vars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Format is `VAR=VALUE`.")
	cmd.PersistentFlags().BoolVarP(&flags.BInsecure, "insecure", "k", false, "Disable all verification of server certificates when sending requests over TLS (HTTPS)")
	cmd.PersistentFlags().BoolVarP(&flags.BQuiet, "quiet", "q", false, "Suppress all unnecessary output.")

	addRequestOutputFlags(cmd)
}

func addQuickMethodCommand(method string) {
	upperMeth := strings.ToUpper(method)
	lowerMeth := strings.ToLower(method)

	var quickCmd = &cobra.Command{
		Use: lowerMeth + " URL",
		Annotations: map[string]string{
			annotationKeyHelpUsages: "" +
				lowerMeth + " URL [-HdCVkbcp] [output-flags]",
		},
		GroupID: "quickreqs",
		Short:   "Make a one-off " + upperMeth + " request",
		Long: "Creates a new one-off" + upperMeth + " request and immediately sends it. No project file is " +
			"consulted, but state files may be read and written. Same as 'morc oneoff -X " + upperMeth + "'",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			var args oneoffArgs
			if err := parseOneoffArgs(cmd, posArgs, &args); err != nil {
				return err
			}

			// done checking args, don't show usage on error
			cmd.SilenceUsage = true
			io := cmdio.From(cmd)
			io.Quiet = flags.BQuiet

			return makeOneoffRequest(io, args)
		},
	}

	addOneoffRequestFlags(quickCmd)
	rootCmd.AddCommand(quickCmd)
}

func makeOneoffRequest(io cmdio.IO, args oneoffArgs) error {
	args.outputCtrl.Writer = io.Out

	sendOpts := morc.SendOptions{
		LoadStateFile:      args.stateFileIn,
		SaveStateFile:      args.stateFileOut,
		Headers:            args.headers,
		Body:               args.bodyData,
		Captures:           args.captures,
		Output:             args.outputCtrl,
		Vars:               args.vars,
		InsecureSkipVerify: args.skipVerify,
	}

	// inject the http client, in case we are to use a specific one
	sendOpts.Client = cmdio.HTTPClient

	_, err := morc.Send(strings.ToUpper(args.method), args.url, args.prefix, sendOpts)
	return err
}

type oneoffArgs struct {
	method string
	url    string

	stateFileIn  string
	stateFileOut string
	vars         map[string]string
	captures     []morc.VarScraper
	headers      http.Header
	bodyData     []byte
	outputCtrl   morc.OutputControl
	skipVerify   bool
	prefix       string
}

func parseOneoffArgs(cmd *cobra.Command, posArgs []string, args *oneoffArgs) error {
	// get method and args from either command name or args
	if len(posArgs) == 1 {
		args.method = cmd.Name()
		args.url = posArgs[0]
	} else {
		args.method = posArgs[0]
		args.url = posArgs[1]
	}

	args.stateFileIn = flags.ReadStateFile
	args.stateFileOut = flags.WriteStateFile
	args.skipVerify = flags.BInsecure
	args.prefix = flags.VarPrefix

	if args.prefix == "" {
		return fmt.Errorf("variable prefix cannot be set to empty string")
	}

	var err error
	args.outputCtrl, err = gatherRequestOutputFlags(cmd)
	if err != nil {
		return err
	}

	// check get vars
	if len(flags.CaptureVars) > 0 {
		scrapers := []morc.VarScraper{}

		for idx, gv := range flags.CaptureVars {
			scraper, err := morc.ParseVarScraper(gv)
			if err != nil {
				return fmt.Errorf("get-var #%d (%q): %w", idx+1, gv, err)
			}
			scrapers = append(scrapers, scraper)
		}

		args.captures = scrapers
	}

	// check vars
	if len(flags.Vars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range flags.Vars {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("var #%d (%q) is not in format key=value", idx+1, v)
			}
			oneTimeVars[parts[0]] = parts[1]
		}
		args.vars = oneTimeVars
	}

	// check headers and load into an http.Header
	if len(flags.Headers) > 0 {
		headers := make(http.Header)
		for idx, h := range flags.Headers {

			// split the header into key and value
			parts := strings.SplitN(h, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("header #%d (%q) is not in format key: value", idx+1, h)
			}
			canonKey := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
			if canonKey == "" {
				return fmt.Errorf("header #%d (%q) does not have a valid header key", idx+1, h)
			}
			value := strings.TrimSpace(parts[1])
			headers.Add(canonKey, value)
		}
		args.headers = headers
	}

	// check body data; load it immediately if it refers to a file
	if strings.HasPrefix(flags.BodyData, "@") {
		// read entire file now
		fRaw, err := os.Open(flags.BodyData[1:])
		if err != nil {
			return fmt.Errorf("open %q: %w", flags.BodyData[1:], err)
		}
		defer fRaw.Close()
		bodyData, err := io.ReadAll(fRaw)
		if err != nil {
			return fmt.Errorf("read %q: %w", flags.BodyData[1:], err)
		}
		args.bodyData = bodyData
	} else {
		args.bodyData = []byte(flags.BodyData)
	}

	return nil
}
