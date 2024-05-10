package reqs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/dekarrin/suyac/cmd/suyac/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagBodyData string
	flagHeaders  []string
	flagMethod   string
	flagURL      string
)

// TODO: swap all project file references to -P.
func init() {
	newCmd.PersistentFlags().StringVarP(&flagBodyData, "data", "d", "", "Add the given data as a body to the request; prefix with @ to read data from a file")
	newCmd.PersistentFlags().StringArrayVarP(&flagHeaders, "header", "H", []string{}, "Add a header to the request")
	newCmd.PersistentFlags().StringVarP(&flagMethod, "method", "X", "GET", "Specify the method to use for the request")
	newCmd.PersistentFlags().StringVarP(&flagURL, "url", "u", "http://example.com", "Specify the URL for the request")

	RootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new REQ [-F project_file] [-H header]... [-d body_data | -d @file] [-X method] [-u url]",
	Short: "Create a new request template",
	Long:  "Create a new request template with options to specify details of it. The template can later be sent by calling 'suyac send NAME'",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		opts := newOptions{
			projFile: commonflags.ReqProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		if len(flagHeaders) > 0 {
			headers := make(http.Header)
			for idx, h := range flagHeaders {

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
			opts.body = bodyData
		} else {
			opts.body = []byte(flagBodyData)
		}

		opts.method = strings.ToUpper(flagMethod)
		opts.url = flagURL

		// add scheme to url if non empty and not present
		if opts.url != "" && !strings.HasPrefix(opts.url, "http://") && !strings.HasPrefix(opts.url, "https://") {
			opts.url = "http://" + opts.url
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqNew(reqName, opts)
	},
}

type newOptions struct {
	projFile string
	body     []byte
	headers  http.Header
	method   string
	url      string
}

func invokeReqNew(name string, opts newOptions) error {
	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)

	// check if the project already has a request with the same name
	if _, exists := p.Templates[name]; exists {
		return fmt.Errorf("request template %s already exists in project", name)
	}

	// create the new request template
	req := suyac.RequestTemplate{
		Name:    name,
		Method:  opts.method,
		URL:     opts.url,
		Headers: opts.headers,
		Body:    opts.body,
	}
	p.Templates[name] = req

	// save the project file
	return p.PersistToDisk(false)
}
