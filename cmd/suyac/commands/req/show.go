package req

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagBodyOnly     bool
	flagMethodOnly   bool
	flagURLOnly      bool
	flagHeadersOnly  bool
	flagCapturesOnly bool
	flagAuthFlowOnly bool
)

func init() {
	showCmd.PersistentFlags().BoolVarP(&flagBodyOnly, "body", "", false, "Show only the body of the request")
	showCmd.PersistentFlags().BoolVarP(&flagMethodOnly, "method", "", false, "Show only the method of the request")
	showCmd.PersistentFlags().BoolVarP(&flagURLOnly, "url", "", false, "Show only the URL of the request")
	showCmd.PersistentFlags().BoolVarP(&flagHeadersOnly, "headers", "", false, "Show only the headers of the request")
	showCmd.PersistentFlags().BoolVarP(&flagCapturesOnly, "captures", "", false, "Show only the var captures of the request")
	showCmd.PersistentFlags().BoolVarP(&flagAuthFlowOnly, "auth", "", false, "Show only the auth flow of the request")
	showCmd.MarkFlagsMutuallyExclusive("body", "method", "url", "headers", "captures", "auth")

	RootCmd.AddCommand(showCmd)
}

var showCmd = &cobra.Command{
	Use:   "show NAME [-P project_file] [--body | --method | --url | --headers | --captures | --auth]",
	Short: "Show details on a request template",
	Long:  "Print out the details of a request template in the project. If no flags are given, prints out all data",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		opts := showOptions{
			projFile: flagProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		if flagBodyOnly {
			opts.show = showBody
		} else if flagMethodOnly {
			opts.show = showMethod
		} else if flagURLOnly {
			opts.show = showURL
		} else if flagHeadersOnly {
			opts.show = showHeaders
		} else if flagCapturesOnly {
			opts.show = showCaptures
		} else if flagAuthFlowOnly {
			opts.show = showAuthFlow
		} else {
			opts.show = showAll
		}

		return invokeReqShow(reqName, opts)
	},
}

type reqShowable int

const (
	showAll reqShowable = iota
	showBody
	showMethod
	showURL
	showHeaders
	showCaptures
	showAuthFlow
)

type showOptions struct {
	projFile string
	show     reqShowable
}

func invokeReqShow(name string, opts showOptions) error {
	// load the project file
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)

	req, ok := p.Templates[name]
	if !ok {
		return fmt.Errorf("no request template %s", name)
	}

	// print out the request details

	if opts.show == showAll {
		meth := req.Method
		if meth == "" {
			meth = "(NO-METHOD)"
		}
		url := req.URL
		if url == "" {
			url = "(NO-URL)"
		}
		fmt.Printf("%s - %s %s\n", req.Name, meth, url)
	} else if opts.show == showMethod {
		if req.Method == "" {
			fmt.Printf("(NONE)\n")
		} else {
			fmt.Printf("%s\n", req.Method)
		}
		return nil
	} else if opts.show == showURL {
		if req.URL == "" {
			fmt.Printf("(NONE)\n")
		} else {
			fmt.Printf("%s\n", req.URL)
		}
		return nil
	}

	if opts.show == showCaptures || opts.show == showAll {
		if len(req.Captures) > 0 {
			if opts.show == showAll {
				fmt.Printf("VAR CAPTURES:\n")
			}
			for _, cap := range req.Captures {
				fmt.Printf("%s\n", cap.String())
			}
		} else {
			if opts.show == showAll {
				fmt.Printf("VAR CAPTURES: (NONE)\n")
			} else {
				fmt.Printf("(NONE)\n")
			}
		}
		if opts.show == showCaptures {
			return nil
		}
	}

	if opts.show == showAuthFlow || opts.show == showAll {
		if req.AuthFlow == "" {
			if opts.show == showAll {
				fmt.Printf("AUTH FLOW: (NONE)\n")
			} else {
				fmt.Printf("(NONE)\n")
			}
		} else {
			if opts.show == showAll {
				fmt.Printf("AUTH FLOW: ")
			}
			fmt.Printf("%s\n", req.AuthFlow)
		}

		if opts.show == showAuthFlow {
			return nil
		}
	}

	// print out headers, if any
	if opts.show == showHeaders || opts.show == showAll {
		if len(req.Headers) > 0 {
			if opts.show == showAll {
				fmt.Printf("HEADERS:\n")
			}

			// alphabetize headers
			var sortedNames []string
			for name := range req.Headers {
				sortedNames = append(sortedNames, name)
			}
			sort.Strings(sortedNames)

			for _, name := range sortedNames {
				for _, val := range req.Headers[name] {
					fmt.Printf("%s: %s\n", name, val)
				}
			}
		} else {
			if opts.show == showAll {
				fmt.Printf("HEADERS: (NONE)\n")
			} else {
				fmt.Printf("(NONE)\n")
			}
		}
		if opts.show == showHeaders {
			return nil
		}
	}

	if opts.show == showBody || opts.show == showAll {
		if len(req.Body) > 0 {
			if opts.show == showAll {
				fmt.Printf("BODY:\n")
			}
			fmt.Printf("%s\n", string(req.Body))
		} else {
			if opts.show == showAll {
				fmt.Printf("BODY: (NONE)\n")
			} else {
				fmt.Printf("(NONE)\n")
			}
		}
		if opts.show == showBody {
			return nil
		}
	}

	return nil
}
