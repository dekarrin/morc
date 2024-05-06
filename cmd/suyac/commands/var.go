package commands

import (
	"fmt"
	"sort"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagVarProjectFile string
	flagVarDelete      bool
	flagVarEnv         string
	flagVarDefaultEnv  bool
)

func init() {
	varCmd.PersistentFlags().StringVarP(&flagVarProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
	varCmd.PersistentFlags().BoolVarP(&flagVarDelete, "delete", "d", false, "Delete the specified variable. Only valid when giving a NAME and no VALUE")
	varCmd.PersistentFlags().StringVarP(&flagVarEnv, "env", "e", "", "Run the command against the given environment instead of the current one. Use --default instead to specify the default environment.")
	varCmd.PersistentFlags().BoolVarP(&flagVarDefaultEnv, "default", "", false, "Run the command against the default environment instead of the current one.")

	// mark the env and default flags as mutually exclusive
	varCmd.MarkFlagsMutuallyExclusive("env", "default")

	rootCmd.AddCommand(varCmd)
}

var varCmd = &cobra.Command{
	Use:     "var [NAME [VALUE]] [-F project_file] [-d] [-e ENV]",
	GroupID: "project",
	Short:   "Show or manipulate request variables",
	Long:    "Prints out a listing of the values of variables if given no other arguments. If given the NAME of a variable only, that variable's value will be printed out. If given the NAME and a VALUE, sets the variable to that value. To delete a variable, -d can be used with the NAME-only form.\n\nIf --env or --default is used, a listing will exclusively show variables defined in that environment, whereas typically it would show values in the current environment, supplemented with those from the default environment for vars that are not defined in the specific one. If the current environment *is* the default environment, there is no distinction.",
	Args:    cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := varOptions{
			projFile:           flagVarProjectFile,
			envOverride:        flagVarEnv,
			envDefaultOverride: flagVarDefaultEnv,
			deleteVar:          flagVarDelete,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// what mode are we in? listing, reading, or writing? infer by arg count
		if len(args) == 0 {
			// listing mode
			if opts.deleteVar {
				return fmt.Errorf("must specify name of variable to delete")
			}

			// otherwise, go ahead and call list
			return invokeVarList(opts)
		} else if len(args) == 1 {
			// value get mode, or a delete
			if opts.deleteVar {
				return invokeVarDelete(args[0], opts)
			}
			return invokeVarGet(args[0], opts)
		} else if len(args) == 2 {
			// value set mode.
			if opts.deleteVar {
				return fmt.Errorf("cannot specify value when using --delete/-d flag")
			}
			return invokeVarSet(args[0], args[1], opts)
		}

		return invokeVarList(opts)
	},
}

type varOptions struct {
	projFile           string
	envOverride        string
	envDefaultOverride bool
	deleteVar          bool
}

func invokeVarList(opts varOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// are we looking to get from a specific environment?
	if opts.envOverride != "" || opts.envDefaultOverride {
		// we want a specific environment only.

		if opts.envDefaultOverride {
			// we want the defaults only.
			p.Vars.Environment = ""
		} else {
			p.Vars.Environment = opts.envOverride
		}

		// run as normal.
		vars := p.Vars.Defined()

		// alphabetize the vars
		sort.Strings(vars)

		if len(vars) == 0 {
			fmt.Println("(none)")
		} else {
			for _, name := range vars {
				fmt.Printf("$%s = %q\n", name, p.Vars.Get(name))
			}
		}
		return nil
	}

	// otherwise, we want ALL the current vars.

	names := p.Vars.EnvNames()

	if len(p.Templates) == 0 {
		fmt.Println("(none)")
	} else {
		// alphabetize the templates
		var sortedNames []string
		for name := range p.Templates {
			sortedNames = append(sortedNames, name)
		}
		sort.Strings(sortedNames)

		for _, name := range sortedNames {
			meth := p.Templates[name].Method
			if meth == "" {
				meth = "???"
			}
			fmt.Printf("%s %s\n", name, meth)
		}
	}

	return nil
}
