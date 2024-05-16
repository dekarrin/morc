package caps

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new REQ VAR CAP [-F project_file]",
	Short: "Add a new variable capture to a request template",
	Long:  "Add a new variable capture to a request template. The capture will be attempted on responses to requests made from this template. VAR must be a variable name containing only letters, numbers, or underscore. CAP must be either ':START,END' for a byte offset (ex: \":4,20\") or a jq-ish path with only keys and variable indexes (ex: \"records[1].auth.token\")",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		varName := args[1]
		if varName == "" {
			return fmt.Errorf("variable name cannot be empty")
		}

		varCap := args[2]
		if varCap == "" {
			return fmt.Errorf("variable capture spec cannot be empty")
		}

		opts := newOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqCapsNew(reqName, varName, varCap, opts)
	},
}

type newOptions struct {
	projFile string
}

func invokeReqCapsNew(name, varName, varCap string, opts newOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	name = strings.ToLower(name)
	req, ok := p.Templates[name]
	if !ok {
		return fmt.Errorf("no request template %s", name)
	}

	// parse the var scraper
	varName, err = morc.ParseVarName(varName)
	if err != nil {
		return err
	}

	// var name normalized to upper case
	varUpper := strings.ToUpper(varName)
	if len(req.Captures) > 0 {
		if _, ok := req.Captures[varUpper]; ok {
			return fmt.Errorf("variable $%s already has a capture", varUpper)
		}
	}

	// parse the capture spec
	scraper, err := morc.ParseVarScraperSpec(varName, varCap)
	if err != nil {
		return err
	}

	// otherwise, we have a valid capture, so add it to the request.
	if req.Captures == nil {
		req.Captures = make(map[string]morc.VarScraper)
		p.Templates[name] = req
	}
	req.Captures[varUpper] = scraper

	// save the project file
	return p.PersistToDisk(false)
}
