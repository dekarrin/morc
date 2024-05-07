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
	flagVarAll         bool
)

func init() {
	varCmd.PersistentFlags().StringVarP(&flagVarProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
	varCmd.PersistentFlags().BoolVarP(&flagVarDelete, "delete", "d", false, "Delete the specified variable. Only valid when giving a NAME and no VALUE")
	varCmd.PersistentFlags().StringVarP(&flagVarEnv, "env", "e", "", "Run the command against the given environment instead of the current one. Use --default instead to specify the default environment.")
	varCmd.PersistentFlags().BoolVarP(&flagVarDefaultEnv, "default", "", false, "Run the command against the default environment instead of the current one.")
	varCmd.PersistentFlags().BoolVarP(&flagVarAll, "all", "", false, "Used with -d. Delete the variable from all environments. This is the only way to effectively specify '--default' while also calling -d; it is a separate flag to indicate that the variable will indeed be erased everywhere, not just in the default environment.")

	// mark the env and default flags as mutually exclusive
	varCmd.MarkFlagsMutuallyExclusive("env", "default", "all")

	rootCmd.AddCommand(varCmd)
}

var varCmd = &cobra.Command{
	Use:     "var [NAME [VALUE]] [-F project_file] [-d [--all]] [-e ENV]|[--default]",
	GroupID: "project",
	Short:   "Show or manipulate request variables",
	Long:    "Prints out a listing of the variables accessible from the current variable environment if given no other arguments. If given the NAME of a variable only, that variable's value will be printed out. If given the NAME and a VALUE, sets the variable to that value. To delete a variable, -d can be used with the NAME-only form.\n\nIf --env or --default is used, a listing will exclusively show variables defined in that environment, whereas typically it would show values in the current environment, supplemented with those from the default environment for vars that are not defined in the specific one. If the current environment *is* the default environment, there is no distinction.",
	Args:    cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := varOptions{
			projFile:           flagVarProjectFile,
			envOverride:        flagVarEnv,
			envDefaultOverride: flagVarDefaultEnv,
			deleteVar:          flagVarDelete,
			envAll:             flagVarAll,
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
			if opts.envAll {
				return fmt.Errorf("--all is only valid when deleting a var")
			}

			// otherwise, go ahead and call list
			return invokeVarList(opts)
		} else if len(args) == 1 {
			// value get mode, or a delete
			if opts.deleteVar {
				if opts.envDefaultOverride {
					return fmt.Errorf("cannot specify --default with --delete/-d; use --all to delete from all environments")
				}
				return invokeVarDelete(args[0], opts)
			}

			if opts.envAll {
				return fmt.Errorf("--all is only valid when deleting a var; use --default to get var's value in default environment")
			}
			return invokeVarGet(args[0], opts)
		} else if len(args) == 2 {
			// value set mode.
			if opts.deleteVar {
				return fmt.Errorf("cannot specify value when using --delete/-d flag")
			}
			if opts.envAll {
				return fmt.Errorf("--all is only valid when deleting; use --default to set var in the default environment")
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
	envAll             bool
	deleteVar          bool
}

func invokeVarSet(varName, value string, opts varOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if opts.envDefaultOverride {
		p.Vars.SetIn(varName, value, "")
	} else if opts.envOverride != "" {
		p.Vars.SetIn(varName, value, opts.envOverride)
	} else {
		p.Vars.Set(varName, value)

	}

	return p.PersistToDisk(false)
}

func invokeVarGet(varName string, opts varOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	var val string
	if opts.envDefaultOverride {
		val = p.Vars.GetFrom(varName, "")
	} else if opts.envOverride != "" {
		val = p.Vars.GetFrom(varName, opts.envOverride)
	} else {
		val = p.Vars.Get(varName)
	}

	fmt.Println(val)

	return nil
}

func invokeVarDelete(varName string, opts varOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// are we looking to delete from a specific environment?
	if opts.envDefaultOverride { // will never be true as user must specify --all to get this behavior
		panic("'envDefaultOverride' option cannot be set when deleting a variable")
	}

	if opts.envAll {
		// easy, just delete from all environments
		p.Vars.Remove(varName)
		return p.PersistToDisk(false)
	}

	inOtherEnv := opts.envOverride != "" && p.Vars.Environment != opts.envOverride

	if inOtherEnv {
		p.Vars.UnsetIn(opts.envOverride, varName)
	} else {
		p.Vars.Unset(varName)
	}

	return p.PersistToDisk(false)
}

func invokeVarList(opts varOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	var vars []string
	// are we looking to get from a specific environment?
	if opts.envOverride != "" || opts.envDefaultOverride {
		// we want a specific environment only.

		// either envDefaultOverride is set, meaning we should use the default,
		// so envOverride will be empty. Or, envOverride will never be empty if
		// envDefaultOverride is not set.
		targetEnv := opts.envOverride
		vars = p.Vars.DefinedIn(targetEnv)
	} else {
		vars = p.Vars.All()
	}

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
