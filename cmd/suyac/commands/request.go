package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagWriteStateFile        string
	flagReadStateFile         string
	flagHeaders               []string
	flagBodyData              string
	flagVarSymbol             string
	flagOutputResponseHeaders bool
)

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Make an arbitrary HTTP request",
	Long:  "Creates a new request and sends it using the specified method. The method may be non-standard.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// check flags while populating opts
		opts := requestOptions{
			stateFileIn:   flagReadStateFile,
			stateFileOut:  flagWriteStateFile,
			outputHeaders: flagOutputResponseHeaders,
		}

		if flagVarSymbol == "" {
			return fmt.Errorf("variable symbol cannot be empty")
		}

		// check headers and load into an http.Header
		if len(flagHeaders) > 0 {
			headers := make(http.Header)
			for idx, h := range flagHeaders {

				// split the header into key and value
				parts := strings.SplitN(h, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("header %d (%q) is not in format key: value", idx, h)
				}
				canonKey := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
				if canonKey == "" {
					return fmt.Errorf("header %d (%q) does not have a valid header key", idx, h)
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
				return fmt.Errorf("open %q: %w", flagBodyData[1:], err)
			}
			defer fRaw.Close()
			bodyData, err := io.ReadAll(fRaw)
			if err != nil {
				return fmt.Errorf("read %q: %w", flagBodyData[1:], err)
			}
			opts.bodyData = bodyData
		} else {
			opts.bodyData = []byte(flagBodyData)
		}

		return invokeRequest(args[0], args[1], flagVarSymbol, opts)
	},
}

func init() {
	requestCmd.PersistentFlags().StringVarP(&flagWriteStateFile, "write-state", "b", "", "Write collected cookies and captured vars to the given file")
	requestCmd.PersistentFlags().StringVarP(&flagReadStateFile, "read-state", "c", "", "Read and use the cookies and vars from the given file")
	requestCmd.PersistentFlags().StringArrayVarP(&flagHeaders, "header", "H", []string{}, "Add a header to the request")
	requestCmd.PersistentFlags().StringVarP(&flagBodyData, "data", "d", "", "Add the given data as a body to the request; prefix with @ to read data from a file")
	requestCmd.PersistentFlags().StringVarP(&flagVarSymbol, "var-symbol", "v", "$", "The symbol to use for variable substitution")
	requestCmd.PersistentFlags().BoolVarP(&flagOutputResponseHeaders, "output-headers", "o", false, "Output the headers of the response")

	rootCmd.AddCommand(requestCmd)
}

type requestOptions struct {
	stateFileOut  string
	stateFileIn   string
	headers       http.Header
	bodyData      []byte
	outputHeaders bool
}

// invokeRequest receives named vars and checked/defaulted requestOptions.
func invokeRequest(method, url, varSymbol string, opts requestOptions) error {
	if varSymbol == "" {
		return fmt.Errorf("variable symbol cannot be empty")
	}

	// create the client
	client := suyac.NewRESTClient(0) // TODO: allow cookie settings

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

	resp, err := client.Request(method, url, opts.bodyData, opts.headers)
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

	// output the status line
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)

	// output the response headers if requested
	if opts.outputHeaders {
		fmt.Println("------------------- HEADERS -------------------")

		// alphabetize the headers
		keys := make([]string, 0, len(resp.Header))
		for k := range resp.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			vals := resp.Header[k]
			for _, v := range vals {
				fmt.Printf("%s: %s\n", k, v)
			}
		}

		fmt.Println("-----------------------------------------------")
	}

	// output the response body, if any
	if resp.Body != nil {
		entireBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
		if len(entireBody) > 0 {
			fmt.Println(string(entireBody))
		}
	}

	return nil
}
