package commands

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"sort"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagProjectFile string
)

func init() {
	sendCmd.PersistentFlags().StringVarP(&flagProjectFile, "project_file", "P", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
	sendCmd.PersistentFlags().BoolVarP(&flagOutputResponseHeaders, "headers", "", false, "Output the headers of the response")
	sendCmd.PersistentFlags().StringVarP(&flagFormat, "format", "f", "pretty", "Output format (pretty, line, sr)")
	sendCmd.PersistentFlags().StringArrayVarP(&flagVars, "var", "V", []string{}, "Temporarily set a variable's value for the current request only. Format is name:value")
	sendCmd.PersistentFlags().BoolVarP(&flagOutputCaptures, "captures", "", false, "Output the captures from the response")
	sendCmd.PersistentFlags().BoolVarP(&flagSuppressResponseBody, "no-body", "", false, "Suppress the output of the response body")
	sendCmd.PersistentFlags().BoolVarP(&flagOutputRequest, "request", "", false, "Output the filled request prior to sending it")
	sendCmd.PersistentFlags().BoolVarP(&flagSuppressResponseBody, "no-body", "", false, "Suppress the output of the response body")

	rootCmd.AddCommand(sendCmd)
}

type sendOptions struct {
	projFile             string
	outputHeaders        bool
	outputCaptures       bool
	outputRequest        bool
	suppressResponseBody bool
	oneTimeVars          map[string]string
	format               format
}

var sendCmd = &cobra.Command{
	Use:   "send REQ [-P project_file]",
	Short: "Send a request defined in a template (req)",
	Long:  "Send a request by building it from a request template (req) stored in the project.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts, err := sendFlagsToOptions()
		if err != nil {
			return err
		}

		return invokeSend(args[0], opts)
	},
}

func sendFlagsToOptions() (sendOptions, error) {
	opts := sendOptions{
		outputHeaders:        flagOutputResponseHeaders,
		outputCaptures:       flagOutputCaptures,
		outputRequest:        flagOutputRequest,
		suppressResponseBody: flagSuppressResponseBody,
	}

	opts.projFile = flagProjectFile
	if opts.projFile == "" {
		return opts, fmt.Errorf("project file is set to empty string")
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

	return opts, nil
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeSend(reqName string, opts sendOptions) error {

	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
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

	if tmpl.Method == "" {
		return fmt.Errorf("request template %s has no method set", reqName)
	}

	if tmpl.URL == "" {
		return fmt.Errorf("request template %s has no URL set", reqName)
	}

	const (
		lineDelimStart = ">>>"
		lineDelimEnd   = "<<<"
	)

	varSymbol := "$"

	// create the client
	client := suyac.NewRESTClient(0) // TODO: allow cookie settings
	client.VarOverrides = opts.oneTimeVars
	client.VarPrefix = varSymbol
	//client.Scrapers = opts.scrapers

	// TODO: cookies glue

	req, err := client.CreateRequest(tmpl.Method, tmpl.URL, tmpl.Body, tmpl.Headers)
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

	// TODO: output cookie glue

	// output the captures if requested
	if opts.outputCaptures {
		if opts.format == formatPretty {
			fmt.Println("----------------- VAR CAPTURES ----------------")
		} else if opts.format == formatLine {
			fmt.Println(lineDelimStart + " VARS")
		}

		for k, v := range caps {
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

	// TODO: PERSIST HISTORY

	return nil
}
