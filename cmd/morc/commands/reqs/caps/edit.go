package caps

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagEditVar  string
	flagEditSpec string
)

func init() {
	editCmd.PersistentFlags().StringVarP(&flagEditVar, "var", "v", "", "Change the variable the captured value is saved to. `VAR` must contain only letters, numbers, or underscore.")
	editCmd.PersistentFlags().StringVarP(&flagEditSpec, "spec", "s", "", "Change the capture spec. `CAP` must be either ':START,END' for a byte offset (ex: \":4,20\") or a jq-ish path with only keys and variable indexes (ex: \"records[1].auth.token\")")

	RootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit REQ VAR [-F project_file] --var VAR --spec CAP",
	Short: "Edit a variable capture in the request template.",
	Long:  "Edit an existing variable capture in the request template by changing the var it captures to or the spec of the capture.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		varName := args[1]
		if varName == "" {
			return fmt.Errorf("variable to change cannot be empty")
		}

		opts := editOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// if altering name, parse it to check it

		varName, err = morc.ParseVarScraperName(varName)
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

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqCapsNew(reqName, varName, varCap, opts)
	},
}

type optional[E any] struct {
	set bool
	v   E
}

type editOptions struct {
	projFile string
	varName  string
	newName  optional[string]
	spec     optional[morc.VarScraper]
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
	varName, err = morc.ParseVarScraperName(varName)
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
