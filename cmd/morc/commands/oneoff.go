package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cliflags"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var (
	flagVarSymbol string
)

func addRequestFlags(id string, cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&cliflags.WriteStateFile, "write-state", "b", "", "Write collected cookies and captured vars to statefile `FILE`.")
	cmd.PersistentFlags().StringVarP(&cliflags.ReadStateFile, "read-state", "c", "", "Read and use the cookies and vars saved in statefile `FILE`.")
	cmd.PersistentFlags().StringArrayVarP(&cliflags.Headers, "header", "H", []string{}, "Add a header to the request. Argument is in form `KEY:VALUE` (spaces after the colon are allowed). May be set multiple times.")
	cmd.PersistentFlags().StringVarP(&cliflags.BodyData, "data", "d", "", "Add the given `DATA` as a body to the request; prefix with '@' to instead interperet DATA as a filename that body data is to be read from.")
	cmd.PersistentFlags().StringVarP(&flagVarSymbol, "var-symbol", "", "$", "Set the leading variable symbol used to indicate the start of a variable in the request to `SYM`.")
	cmd.PersistentFlags().StringArrayVarP(&cliflags.CaptureVars, "capture-var", "C", []string{}, "Get a variable's value from the response. Argument is in format `VAR:SPEC`. The SPEC part has format ':START,END' for byte offset (note the leading colon, resulting in 'VAR::START,END'), or 'path[0].to.value' (jq-ish syntax) for JSON body data.")
	cmd.PersistentFlags().StringArrayVarP(&cliflags.Vars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Format is `VAR=VALUE`.")
	cmd.PersistentFlags().BoolVarP(&cliflags.BInsecure, "insecure", "k", false, "Disable all verification of server certificates when sending requests over TLS (HTTPS)")

	setupRequestOutputFlags(id, cmd)
}

type oneoffOptions struct {
	stateFileOut string
	stateFileIn  string
	headers      http.Header
	bodyData     []byte
	oneTimeVars  map[string]string
	scrapers     []morc.VarScraper
	outputCtrl   morc.OutputControl
	skipVerify   bool
}

var oneoffCmd = &cobra.Command{
	Use: "oneoff METHOD URL",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"oneoff METHOD URL [-HdCVkbc] [output-flags]",
	},
	GroupID: "sending",
	Short:   "Make an arbitrary one-off HTTP request",
	Long: "Creates a new request and sends it using the specified method. The method may be non-standard. No " +
		"project file is consulted, but state files may be read and written.",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := oneoffFlagsToOptions("morc oneoff")
		if err != nil {
			return err
		}

		// make sure that the method is upper case
		args[0] = strings.ToUpper(args[0])

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		return invokeRequest(io, args[0], args[1], flagVarSymbol, opts)
	},
}

func init() {
	addRequestFlags("morc oneoff", oneoffCmd)
	rootCmd.AddCommand(oneoffCmd)

	quickMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE"}
	for _, meth := range quickMethods {
		addQuickMethodCommand(meth)
	}
}

func addQuickMethodCommand(method string) {
	upperMeth := strings.ToUpper(method)
	lowerMeth := strings.ToLower(method)

	var quickCmd = &cobra.Command{
		Use: lowerMeth + " URL",
		Annotations: map[string]string{
			annotationKeyHelpUsages: "" +
				lowerMeth + " URL [-HdCVkbc] [output-flags]",
		},
		GroupID: "quickreqs",
		Short:   "Make a one-off " + upperMeth + " request",
		Long: "Creates a new one-off" + upperMeth + " request and immediately sends it. No project file is " +
			"consulted, but state files may be read and written. Same as 'morc oneoff -X " + upperMeth + "'",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := oneoffFlagsToOptions("morc " + lowerMeth)
			if err != nil {
				return err
			}

			// done checking args, don't show usage on error
			cmd.SilenceUsage = true
			io := cmdio.From(cmd)

			return invokeRequest(io, upperMeth, args[0], flagVarSymbol, opts)
		},
	}

	addRequestFlags("morc "+lowerMeth, quickCmd)
	rootCmd.AddCommand(quickCmd)
}

func oneoffFlagsToOptions(cmdID string) (oneoffOptions, error) {
	opts := oneoffOptions{
		stateFileIn:  cliflags.ReadStateFile,
		stateFileOut: cliflags.WriteStateFile,
		skipVerify:   cliflags.BInsecure,
	}

	if flagVarSymbol == "" {
		return opts, fmt.Errorf("variable symbol cannot be empty")
	}

	var err error
	opts.outputCtrl, err = gatherRequestOutputFlags(cmdID)
	if err != nil {
		return opts, err
	}

	// check get vars
	if len(cliflags.CaptureVars) > 0 {
		scrapers := []morc.VarScraper{}

		for idx, gv := range cliflags.CaptureVars {
			scraper, err := morc.ParseVarScraper(gv)
			if err != nil {
				return opts, fmt.Errorf("get-var #%d (%q): %w", idx+1, gv, err)
			}
			scrapers = append(scrapers, scraper)
		}

		opts.scrapers = scrapers
	}

	// check vars
	if len(cliflags.Vars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range cliflags.Vars {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) != 2 {
				return opts, fmt.Errorf("var #%d (%q) is not in format key=value", idx+1, v)
			}
			oneTimeVars[parts[0]] = parts[1]
		}
		opts.oneTimeVars = oneTimeVars
	}

	// check headers and load into an http.Header
	if len(cliflags.Headers) > 0 {
		headers := make(http.Header)
		for idx, h := range cliflags.Headers {

			// split the header into key and value
			parts := strings.SplitN(h, ":", 2)
			if len(parts) != 2 {
				return opts, fmt.Errorf("header #%d (%q) is not in format key: value", idx+1, h)
			}
			canonKey := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
			if canonKey == "" {
				return opts, fmt.Errorf("header #%d (%q) does not have a valid header key", idx+1, h)
			}
			value := strings.TrimSpace(parts[1])
			headers.Add(canonKey, value)
		}
		opts.headers = headers
	}

	// check body data; load it immediately if it refers to a file
	if strings.HasPrefix(cliflags.BodyData, "@") {
		// read entire file now
		fRaw, err := os.Open(cliflags.BodyData[1:])
		if err != nil {
			return opts, fmt.Errorf("open %q: %w", cliflags.BodyData[1:], err)
		}
		defer fRaw.Close()
		bodyData, err := io.ReadAll(fRaw)
		if err != nil {
			return opts, fmt.Errorf("read %q: %w", cliflags.BodyData[1:], err)
		}
		opts.bodyData = bodyData
	} else {
		opts.bodyData = []byte(cliflags.BodyData)
	}

	return opts, nil
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeRequest(io cmdio.IO, method, url, varSymbol string, opts oneoffOptions) error {
	opts.outputCtrl.Writer = io.Out

	sendOpts := morc.SendOptions{
		LoadStateFile:      opts.stateFileIn,
		SaveStateFile:      opts.stateFileOut,
		Headers:            opts.headers,
		Body:               opts.bodyData,
		Captures:           opts.scrapers,
		Output:             opts.outputCtrl,
		Vars:               opts.oneTimeVars,
		InsecureSkipVerify: opts.skipVerify,
	}

	// inject the http client, in case we are to use a specific one
	sendOpts.Client = cmdio.HTTPClient

	_, err := morc.Send(method, url, varSymbol, sendOpts)
	return err
}
