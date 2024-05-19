package caps

import (
	"fmt"

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
			newName, err := morc.ParseVarName(flagEditVar)
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
