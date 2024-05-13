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
	editCmd.PersistentFlags().StringVarP(&flagEditVar, "var", "V", "", "Change the variable the captured value is saved to. `VAR` must contain only letters, numbers, or underscore.")
	editCmd.PersistentFlags().StringVarP(&flagEditSpec, "spec", "s", "", "Change the capture spec. `CAP` must be either ':START,END' for a byte offset (ex: \":4,20\") or a jq-ish path with only keys and variable indexes (ex: \"records[1].auth.token\")")

	RootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit REQ VAR [-F project_file] [--var VAR] [--spec CAP]",
	Short: "Edit a variable capture in the request template.",
	Long:  "Edit an existing variable capture in the request template by changing the var it captures to or the spec of the capture.",
	Args:  cobra.ExactArgs(2),
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

		// check if var flag was set
		if cmd.Flags().Changed("var") {
			if flagEditVar == "" {
				return fmt.Errorf("variable name cannot be empty")
			}
			newName, err := morc.ParseVarScraperName(flagEditVar)
			if err != nil {
				return fmt.Errorf("var: %w", err)
			}
			opts.newName = optional[string]{set: true, v: newName}
		}

		// check if spec flag was set
		if cmd.Flags().Changed("spec") {
			if flagEditSpec == "" {
				return fmt.Errorf("capture spec cannot be set to empty")
			}

			// which var name are we using? new, or old?
			specVarName := varName
			if opts.newName.set {
				specVarName = opts.newName.v
			}

			scraper, err := morc.ParseVarScraperSpec(specVarName, flagEditSpec)
			if err != nil {
				return fmt.Errorf("spec: %w", err)
			}
			opts.spec = optional[morc.VarScraper]{set: true, v: scraper}
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqCapsEdit(reqName, varName, opts)
	},
}

type optional[E any] struct {
	set bool
	v   E
}

type editOptions struct {
	projFile string
	newName  optional[string]
	spec     optional[morc.VarScraper]
}

func invokeReqCapsEdit(reqName, varName string, opts editOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)
	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	// case doesn't matter for var names
	varUpper := strings.ToUpper(varName)

	if len(req.Captures) == 0 {
		return fmt.Errorf("no capture to variable $%s exists in request %s", varUpper, reqName)
	}

	cap, ok := req.Captures[varUpper]
	if !ok {
		return fmt.Errorf("no capture to variable $%s exists in request %s", varUpper, reqName)
	}

	// okay did the user actually ask to change somefin
	if !opts.newName.set && !opts.spec.set {
		return fmt.Errorf("no changes requested")
	}

	// if we have a name change, apply that first
	if opts.newName.set {
		newNameUpper := strings.ToUpper(opts.newName.v)

		// if new name same as old, no reason to do additional work
		if newNameUpper != varUpper {
			// check if the new name is already in use
			if _, ok := req.Captures[newNameUpper]; ok {
				return fmt.Errorf("capture to variable $%s already exists in request %s", opts.newName.v, reqName)
			}

			// remove the old name
			delete(req.Captures, varUpper)

			// add the new one; we will update the name when we save it back to
			// the project
			cap.Name = opts.newName.v
		}
	}

	// if we have a spec change, apply that next
	if opts.spec.set {
		curName := cap.Name
		cap = opts.spec.v
		cap.Name = curName
	}

	// update the request
	req.Captures[strings.ToUpper(cap.Name)] = cap
	p.Templates[reqName] = req

	// save the project file
	return p.PersistToDisk(false)
}
