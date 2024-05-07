package commands

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"sort"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

type format int

const (
	formatPretty format = iota
	formatLine
)

var (
	flagWriteStateFile        string
	flagReadStateFile         string
	flagHeaders               []string
	flagBodyData              string
	flagVarSymbol             string
	flagOutputResponseHeaders bool
	flagOutputCaptures        bool
	flagOutputRequest         bool
	flagSuppressResponseBody  bool
	flagGetVars               []string
	flagVars                  []string
	flagFormat                string
)

func addRequestFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&flagWriteStateFile, "write-state", "b", "", "Write collected cookies and captured vars to the given file")
	cmd.PersistentFlags().StringVarP(&flagReadStateFile, "read-state", "c", "", "Read and use the cookies and vars from the given file")
	cmd.PersistentFlags().StringArrayVarP(&flagHeaders, "header", "H", []string{}, "Add a header to the request")
	cmd.PersistentFlags().StringVarP(&flagBodyData, "data", "d", "", "Add the given data as a body to the request; prefix with @ to read data from a file")
	cmd.PersistentFlags().StringVarP(&flagVarSymbol, "var-symbol", "", "$", "The symbol to use for variable substitution")
	cmd.PersistentFlags().BoolVarP(&flagOutputResponseHeaders, "headers", "", false, "Output the headers of the response")
	cmd.PersistentFlags().BoolVarP(&flagOutputCaptures, "captures", "", false, "Output the captures from the response")
	cmd.PersistentFlags().BoolVarP(&flagSuppressResponseBody, "no-body", "", false, "Suppress the output of the response body")
	cmd.PersistentFlags().BoolVarP(&flagOutputRequest, "request", "", false, "Output the filled request prior to sending it")
	cmd.PersistentFlags().StringArrayVarP(&flagGetVars, "capture-var", "C", []string{}, "Get a variable's value from the response. Format is name::start,end for byte offset or name:path[0].to.value (jq-ish syntax)")
	cmd.PersistentFlags().StringArrayVarP(&flagVars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Format is name:value")
	cmd.PersistentFlags().StringVarP(&flagFormat, "format", "f", "pretty", "Output format (pretty, line, sr)")
}

type requestOptions struct {
	stateFileOut         string
	stateFileIn          string
	headers              http.Header
	bodyData             []byte
	outputHeaders        bool
	outputCaptures       bool
	outputRequest        bool
	suppressResponseBody bool
	oneTimeVars          map[string]string
	scrapers             []suyac.VarScraper
	format               format
}

var requestCmd = &cobra.Command{
	Use:     "request",
	GroupID: "sending",
	Short:   "Make an arbitrary HTTP request",
	Long:    "Creates a new request and sends it using the specified method. The method may be non-standard.",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := requestFlagsToOptions()
		if err != nil {
			return err
		}

		// make sure that the method is upper case
		args[0] = strings.ToUpper(args[0])

		// make sure the URL has a scheme
		lowerURL := strings.ToLower(args[1])
		if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
			args[1] = "http://" + args[1]
		}

		return invokeRequest(args[0], args[1], flagVarSymbol, opts)
	},
}

func init() {
	addRequestFlags(requestCmd)
	rootCmd.AddCommand(requestCmd)

	quickMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE"}
	for _, meth := range quickMethods {
		addQuickMethodCommand(meth)
	}
}

func addQuickMethodCommand(method string) {
	upperMeth := strings.ToUpper(method)
	lowerMeth := strings.ToLower(method)

	var quickCmd = &cobra.Command{
		Use:     lowerMeth,
		GroupID: "quickreqs",
		Short:   "Make a one-off " + upperMeth + " request",
		Long:    "Creates a new one-off" + upperMeth + " request and immediately sends it. No project file is consulted, but state files may be read and written.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := requestFlagsToOptions()
			if err != nil {
				return err
			}

			// make sure the URL has a scheme
			lowerURL := strings.ToLower(args[0])
			if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
				args[0] = "http://" + args[0]
			}

			return invokeRequest(upperMeth, args[0], flagVarSymbol, opts)
		},
	}

	addRequestFlags(quickCmd)
	rootCmd.AddCommand(quickCmd)
}

func requestFlagsToOptions() (requestOptions, error) {
	opts := requestOptions{
		stateFileIn:          flagReadStateFile,
		stateFileOut:         flagWriteStateFile,
		outputHeaders:        flagOutputResponseHeaders,
		outputCaptures:       flagOutputCaptures,
		outputRequest:        flagOutputRequest,
		suppressResponseBody: flagSuppressResponseBody,
	}

	if flagVarSymbol == "" {
		return opts, fmt.Errorf("variable symbol cannot be empty")
	}

	// check format
	switch strings.ToLower(flagFormat) {
	case "pretty":
		opts.format = formatPretty
	case "sr":
		opts.format = formatLine

		// check if user is trying to turn on things that aren't allowed
		if flagOutputRequest || flagOutputResponseHeaders || flagSuppressResponseBody || flagOutputCaptures {
			return opts, fmt.Errorf("format 'sr' only allows status line and response body; use format 'line' for control over output")
		}
	case "line":
		opts.format = formatLine
	default:
		return opts, fmt.Errorf("invalid format %q; must be one of pretty, line, or sr", flagFormat)
	}

	// check get vars
	if len(flagGetVars) > 0 {
		scrapers := []suyac.VarScraper{}

		for idx, gv := range flagGetVars {
			scraper, err := suyac.ParseVarScraper(gv)
			if err != nil {
				return opts, fmt.Errorf("get-var #%d (%q): %w", idx+1, gv, err)
			}
			scrapers = append(scrapers, scraper)
		}

		opts.scrapers = scrapers
	}

	// check vars
	if len(flagVars) > 0 {
		oneTimeVars := make(map[string]string)
		for idx, v := range flagVars {
			parts := strings.SplitN(v, ":", 2)
			if len(parts) != 2 {
				return opts, fmt.Errorf("var #%d (%q) is not in format key: value", idx+1, v)
			}
			oneTimeVars[parts[0]] = parts[1]
		}
		opts.oneTimeVars = oneTimeVars
	}

	// check headers and load into an http.Header
	if len(flagHeaders) > 0 {
		headers := make(http.Header)
		for idx, h := range flagHeaders {

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
	if strings.HasPrefix(flagBodyData, "@") {
		// read entire file now
		fRaw, err := os.Open(flagBodyData[1:])
		if err != nil {
			return opts, fmt.Errorf("open %q: %w", flagBodyData[1:], err)
		}
		defer fRaw.Close()
		bodyData, err := io.ReadAll(fRaw)
		if err != nil {
			return opts, fmt.Errorf("read %q: %w", flagBodyData[1:], err)
		}
		opts.bodyData = bodyData
	} else {
		opts.bodyData = []byte(flagBodyData)
	}

	return opts, nil
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeRequest(method, url, varSymbol string, opts requestOptions) error {
	const (
		lineDelimStart = ">>>"
		lineDelimEnd   = "<<<"
	)

	if varSymbol == "" {
		return fmt.Errorf("variable symbol cannot be empty")
	}

	// create the client
	client := suyac.NewRESTClient(0) // TODO: allow cookie settings
	client.VarOverrides = opts.oneTimeVars
	client.VarPrefix = varSymbol
	client.Scrapers = opts.scrapers

	// if we have been asked to load state, do that now
	if opts.stateFileIn != "" {
		// open the state file and load it
		stateIn, err := os.Open(opts.stateFileIn)
		if err != nil {
			return fmt.Errorf("open state file: %w", err)
		}
		defer stateIn.Close()

		if err := client.ReadState(stateIn); err != nil {
			return fmt.Errorf("read state file: %w", err)
		}
	}

	req, err := client.CreateRequest(method, url, opts.bodyData, opts.headers)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if opts.outputRequest {
		reqBytes, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			return fmt.Errorf("dump request: %w", err)
		}

		if opts.format == formatPretty {
			fmt.Println("------------------- REQUEST -------------------")
		} else if opts.format == formatLine {
			fmt.Println(lineDelimStart + " REQUEST")
		}

		fmt.Println(string(reqBytes))

		if opts.format == formatPretty && req.Body == nil || req.Body == http.NoBody {
			fmt.Println("(no request body)")
		}

		if opts.format == formatPretty {
			fmt.Println("----------------- END REQUEST -----------------")
		} else if opts.format == formatLine {
			fmt.Println(lineDelimEnd)
		}
	}

	resp, caps, err := client.SendRequest(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	// if we have been asked to save state, do that now
	if opts.stateFileOut != "" {
		// open the state file and save it
		stateOut, err := os.Create(opts.stateFileOut)
		if err != nil {
			return fmt.Errorf("create state file: %w", err)
		}
		defer stateOut.Close()

		if err := client.WriteState(stateOut); err != nil {
			return fmt.Errorf("write state file: %w", err)
		}
	}

	// output the captures if requested
	if opts.outputCaptures {
		if opts.format == formatPretty {
			fmt.Println("----------------- VAR CAPTURES ----------------")
		} else if opts.format == formatLine {
			fmt.Println(lineDelimStart + " VARS")
		}

		capNames := []string{}
		for k := range caps {
			capNames = append(capNames, k)
		}

		sort.Strings(capNames)

		for _, k := range capNames {
			v := caps[k]
			if opts.format == formatPretty {
				fmt.Printf("%s: %s\n", k, v)
			} else if opts.format == formatLine {
				fmt.Printf("%s %s\n", k, v)
			}
		}

		if opts.format == formatPretty {
			fmt.Println("-----------------------------------------------")
		} else if opts.format == formatLine {
			fmt.Println(lineDelimEnd)
		}
	}

	// output the status line
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)

	// output the response headers if requested
	if opts.outputHeaders {
		if opts.format == formatPretty {
			fmt.Println("------------------- HEADERS -------------------")
		} else if opts.format == formatLine {
			fmt.Println(lineDelimStart + " HEADERS")
		}

		// alphabetize the headers
		keys := make([]string, 0, len(resp.Header))
		for k := range resp.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			vals := resp.Header[k]
			for _, v := range vals {
				// works for both pretty and line formats
				fmt.Printf("%s: %s\n", k, v)
			}
		}

		if opts.format == formatPretty {
			fmt.Println("-----------------------------------------------")
		} else if opts.format == formatLine {
			fmt.Println(lineDelimEnd)
		}
	}

	// output the response body, if any
	if !opts.suppressResponseBody {
		if resp.Body != nil && resp.Body != http.NoBody {
			entireBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("read response body: %w", err)
			}

			// works for both pretty and line formats
			fmt.Println(string(entireBody))
		} else {
			if opts.format == formatPretty {
				fmt.Println("(no response body)")
			}
		}
	}

	return nil
}
