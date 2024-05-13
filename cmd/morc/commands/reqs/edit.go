package reqs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagEditBodyData   string
	flagEditAddHeaders []string
	flagEditDelHeaders []string
	flagEditDelBody    bool
	flagEditMethod     string
	flagEditURL        string
	flagEditName       string
)

// TODO: swap all project file references to -P.
func init() {
	editCmd.PersistentFlags().StringVarP(&flagEditBodyData, "data", "d", "", "Add the given data as a body to the request; prefix with @ to read data from a file")
	editCmd.PersistentFlags().BoolVarP(&flagEditDelBody, "delete-data", "D", false, "Delete all existing body data from the request")
	editCmd.PersistentFlags().StringArrayVarP(&flagEditAddHeaders, "header", "H", []string{}, "Add header `HDR` to the request. Must be in the format 'key: value'")
	editCmd.PersistentFlags().StringArrayVarP(&flagEditDelHeaders, "remove-header", "r", []string{}, "Remove header with key `KEY` from the request. If multiple headers with the same key exist, only the most recently added one will be deleted.")
	editCmd.PersistentFlags().StringVarP(&flagEditMethod, "method", "X", "GET", "Specify the method to use for the request")
	editCmd.PersistentFlags().StringVarP(&flagEditURL, "url", "u", "http://example.com", "Specify the URL for the request")
	editCmd.PersistentFlags().StringVarP(&flagEditName, "name", "n", "", "Change the name of the request template to `NAME`")

	editCmd.MarkFlagsMutuallyExclusive("data", "delete-data")

	RootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit REQ [-F project_file] [--name NAME] [-r KEY]... [-H HDR]... [-d body_data | -d @file | -D] [-X method] [-u url]",
	Short: "Edit an existing request template",
	Long:  "Edit the details of a request template using CLI options. If the name is changed, all references to the request name in history and in flows will be updated to match. If headers are both deleted and added, ones to delete will be processed first.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		opts := editOptions{
			projFile:   commonflags.ProjectFile,
			deleteBody: flagEditDelBody,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		if len(flagEditAddHeaders) > 0 {
			headers := make(http.Header)
			for idx, h := range flagEditAddHeaders {

				// split the header into key and value
				parts := strings.SplitN(h, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("header add #%d (%q) is not in format key: value", idx+1, h)
				}
				canonKey := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
				if canonKey == "" {
					return fmt.Errorf("header add #%d (%q) does not have a valid header key", idx+1, h)
				}
				value := strings.TrimSpace(parts[1])
				headers.Add(canonKey, value)
			}
			opts.headers = optional[http.Header]{set: true, v: headers}
		}

		if len(flagEditDelHeaders) > 0 {
			// check format of each header key; basically, just make sure there
			// no spaces or colons
			delHeaders := make([]string, len(flagEditDelHeaders))
			for idx, h := range flagEditDelHeaders {
				trimmed := strings.TrimSpace(h)
				if strings.Contains(trimmed, " ") || strings.Contains(trimmed, ":") {
					return fmt.Errorf("header delete #%d (%q) is not a valid header key", idx+1, h)
				}
				delHeaders[idx] = trimmed
			}

			opts.deleteHeaders = optional[[]string]{set: true, v: delHeaders}
		}

		// check body data; load it immediately if it refers to a file
		if cmd.Flags().Changed("data") {
			if strings.HasPrefix(flagEditBodyData, "@") {
				// read entire file now
				fRaw, err := os.Open(flagEditBodyData[1:])
				if err != nil {
					return fmt.Errorf("open %q: %w", flagEditBodyData[1:], err)
				}
				defer fRaw.Close()
				bodyData, err := io.ReadAll(fRaw)
				if err != nil {
					return fmt.Errorf("read %q: %w", flagEditBodyData[1:], err)
				}
				opts.body = optional[[]byte]{set: true, v: bodyData}
			} else {
				opts.body = optional[[]byte]{set: true, v: []byte(flagEditBodyData)}
			}
		}

		// check new request name; this is not allowed to contain spaces
		if cmd.Flags().Changed("name") {
			newName := strings.TrimSpace(flagEditName)
			if newName == "" {
				return fmt.Errorf("request name cannot be set to the empty string")
			}
			if strings.Contains(newName, " ") {
				return fmt.Errorf("request name cannot contain spaces")
			}

			opts.newName = optional[string]{set: true, v: newName}
		}

		// these ones CAN be set to empty. it's not recommended, but it's the
		// default state of a new request template, ergo valid.
		if cmd.Flags().Changed("method") {
			opts.method = optional[string]{set: true, v: strings.ToUpper(flagEditMethod)}
		}
		if cmd.Flags().Changed("url") {
			newURL := flagEditURL

			// add scheme to url if non empty and not present
			lowerURL := strings.ToLower(newURL)
			if newURL != "" && !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
				newURL = "http://" + newURL
			}

			opts.url = optional[string]{set: true, v: newURL}
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqEdit(reqName, opts)
	},
}

type optional[E any] struct {
	set bool
	v   E
}

type editOptions struct {
	projFile      string
	newName       optional[string]
	body          optional[[]byte]
	deleteBody    bool
	headers       optional[http.Header]
	deleteHeaders optional[[]string]
	method        optional[string]
	url           optional[string]
}

func invokeReqEdit(name string, opts editOptions) error {
	// first, are we updating the name? this determines whether we need to load
	// and save history
	loadAllFiles := opts.newName.set

	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, loadAllFiles)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)

	req, ok := p.Templates[name]
	if !ok {
		return fmt.Errorf("no request template %s exists in project", name)
	}

	// if changing names, do that first
	if opts.newName.set {
		newName := strings.ToLower(opts.newName.v)
		if _, exists := p.Templates[newName]; exists {
			return fmt.Errorf("request template %s already exists in project", newName)
		}

		// update the name in the history
		for idx, h := range p.History {
			if h.Template == name {
				p.History[idx].Template = newName
			}
		}

		// update the name in the flows
		for flowName, flow := range p.Flows {
			for idx, step := range flow.Steps {
				if step.Template == name {
					p.Flows[flowName].Steps[idx].Template = newName
				}
			}
		}

		// update the name in the project
		req.Name = newName
		delete(p.Templates, name)
	}

	// any name changes will have gone to req.Name at this point; be shore to
	// use that for any name displaying from this point forward

	// body modifications
	if opts.deleteBody {
		req.Body = nil
	} else if opts.body.set {
		req.Body = opts.body.v
	}

	// header modifications
	if opts.deleteHeaders.set {
		for _, key := range opts.deleteHeaders.v {
			vals := req.Headers.Values(key)
			if len(vals) < 1 {
				return fmt.Errorf("no values for header %q are in request %s", key, req.Name)
			}

			// delete the most recently added header with this key.
			req.Headers.Del(key)

			// if there's more than one value, put the other ones back to honor
			// deleting only the most recent one
			if len(vals) > 1 {
				for _, v := range vals[:len(vals)-1] {
					req.Headers.Add(key, v)
				}
			}

			// okay deletions are now done
		}
	}
	if opts.headers.set {
		for key, vals := range opts.headers.v {
			for _, v := range vals {
				req.Headers.Add(key, v)
			}
		}
	}

	// method and URL modifications
	if opts.method.set {
		req.Method = opts.method.v
	}
	if opts.url.set {
		req.URL = opts.url.v
	}

	p.Templates[req.Name] = req

	// save the project file
	return p.PersistToDisk(loadAllFiles)
}
