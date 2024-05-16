package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

const (
	reservedDefaultEnvName = "<DEFAULT>"
)

var (
	flagVarsProjectFile string
	flagVarsDelete      bool
	flagVarsEnv         string
	flagVarsDefaultEnv  bool
	flagVarsAll         bool
	flagVarsCurrent     bool
)

type varsAction int

const (
	varsActionList varsAction = iota
	varsActionGet
	varsActionSet
	varsActionDelete
)

func init() {
	varsCmd.PersistentFlags().StringVarP(&flagVarsProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	varsCmd.PersistentFlags().BoolVarP(&flagVarsDelete, "delete", "d", false, "Delete the specified variable. Only valid when giving a NAME and no VALUE")
	varsCmd.PersistentFlags().StringVarP(&flagVarsEnv, "env", "e", "", "Run the command against the given environment instead of the current one. Use --default instead to specify the default environment.")
	varsCmd.PersistentFlags().BoolVarP(&flagVarsDefaultEnv, "default", "", false, "Run the command against the default environment instead of the current one.")
	varsCmd.PersistentFlags().BoolVarP(&flagVarsCurrent, "current", "", false, "Apply only to current environment. This is the same as typing --env followed by the name of the current environment.")
	varsCmd.PersistentFlags().BoolVarP(&flagVarsAll, "all", "", false, "Used with -d. Delete the variable from all environments. This is the only way to effectively specify '--default' while also calling -d; it is a separate flag to indicate that the variable will indeed be erased everywhere, not just in the default environment.")

	// mark the env and default flags as mutually exclusive
	varsCmd.MarkFlagsMutuallyExclusive("env", "default", "all", "current")

	rootCmd.AddCommand(varsCmd)
}

var varsCmd = &cobra.Command{
	Use:     "vars [NAME [VALUE]] [-F project_file] [-d [--all]] [-e ENV]|[--default]",
	GroupID: "project",
	Short:   "Show or manipulate request variables",
	Long:    "Prints out a listing of the variables accessible from the current variable environment (which includes any from default environment, not specifically set in current, unless --current or --env or --default is given) if given no other arguments. If given the NAME of a variable only, that variable's value will be printed out. If given the NAME and a VALUE, sets the variable to that value. To delete a variable, -d can be used with the NAME-only form.\n\nIf --env or --default is used, a listing will exclusively show variables defined in that environment, whereas typically it would show values in the current environment, supplemented with those from the default environment for vars that are not defined in the specific one. If the current environment *is* the default environment, there is no distinction.",
	Args:    cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := varOptions{
			projFile:           flagVarsProjectFile,
			envOverride:        flagVarsEnv,
			envDefaultOverride: flagVarsDefaultEnv,
			envCurrentOverride: flagVarsCurrent,
			deleteVar:          flagVarsDelete,
			envAll:             flagVarsAll,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		var varName string
		var varValue string
		var err error
		action := varsActionList

		// what mode are we in? listing, reading, or writing? infer by arg count
		if len(args) == 0 {
			// listing mode

			if opts.deleteVar {
				return fmt.Errorf("must specify name of variable to delete")
			}
			if opts.envAll {
				return fmt.Errorf("--all is only valid when deleting a var")
			}
			if opts.envOverride == reservedDefaultEnvName {
				return fmt.Errorf("cannot use reserved environment name %q; use --default to list vars in default env", reservedDefaultEnvName)
			}

			// otherwise, go ahead and call list
			action = varsActionList
		} else if len(args) == 1 {
			// value get mode, or a delete

			varName = args[0]

			// don't check varName; deletion and getting should always be
			// allowed in case the project got into a weird state, say due to
			// direct editing of the project file.

			if opts.deleteVar {
				if opts.envDefaultOverride {
					return fmt.Errorf("cannot specify --default with --delete/-d; use --all to delete from all envs")
				}
				if opts.envOverride == reservedDefaultEnvName {
					return fmt.Errorf("cannot use reserved environment name %q; use --all to delete from all envs (including default)", reservedDefaultEnvName)
				}

				action = varsActionDelete
			} else {
				// don't check varName; getting should always be allowed in
				// case the project got into a weird state, say due to direct
				// editing of the project file.

				if err != nil {
					return err
				}

				if opts.envOverride == reservedDefaultEnvName {
					return fmt.Errorf("cannot use reserved environment name %q; use --default to get the default env's value", reservedDefaultEnvName)
				}

				if opts.envAll {
					return fmt.Errorf("--all is only valid when deleting a var; use --default to get from default env")
				}

				action = varsActionGet
			}
		} else if len(args) == 2 {
			// value set mode
			varName = args[0]
			varValue = args[1]

			if opts.deleteVar {
				return fmt.Errorf("cannot specify value when using --delete/-d flag")
			}
			if opts.envAll {
				return fmt.Errorf("--all is only valid when deleting; use --default to set var in the default environment")
			}
			if opts.envOverride == reservedDefaultEnvName {
				return fmt.Errorf("cannot use reserved environment name %q; use --default to set in default env", reservedDefaultEnvName)
			}
			action = varsActionSet
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch action {
		case varsActionList:
			return invokeVarList(io, opts)
		case varsActionGet:
			return invokeVarGet(io, varName, opts)
		case varsActionSet:
			return invokeVarSet(io, varName, varValue, opts)
		case varsActionDelete:
			return invokeVarDelete(io, varName, opts)
		default:
			panic(fmt.Sprintf("unhandled var action %q", action))
		}
	},
}

type varOptions struct {
	projFile           string
	envOverride        string
	envDefaultOverride bool
	envCurrentOverride bool
	envAll             bool
	deleteVar          bool
}

func invokeVarSet(_ cmdio.IO, varName, value string, opts varOptions) error {
	// dont even bother to load if the var name is invalid
	varName, err := morc.ParseVarName(strings.ToUpper(varName))
	if err != nil {
		return err
	}

	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if opts.envDefaultOverride {
		p.Vars.SetIn(varName, value, "")
	} else if opts.envOverride != "" {
		p.Vars.SetIn(varName, value, opts.envOverride)
	} else if opts.envCurrentOverride {
		p.Vars.SetIn(varName, value, p.Vars.Environment)
	} else {
		p.Vars.Set(varName, value)

	}

	return p.PersistToDisk(false)
}

func invokeVarGet(io cmdio.IO, varName string, opts varOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	var val string
	if opts.envDefaultOverride {
		if !p.Vars.IsDefinedIn(varName, "") {
			io.PrintErrf("%q is not defined in default environment\n", varName)
			return nil
		}

		val = p.Vars.GetFrom(varName, "")
	} else if opts.envOverride != "" {
		if !p.Vars.IsDefinedIn(varName, opts.envOverride) {
			io.PrintErrf("%q is not defined in environment %q\n", varName, opts.envOverride)
			return nil
		}

		val = p.Vars.GetFrom(varName, opts.envOverride)
	} else if opts.envCurrentOverride {
		if !p.Vars.IsDefinedIn(varName, p.Vars.Environment) {
			io.PrintErrf("%q is not defined in current environment (%q)\n", varName, p.Vars.Environment)
			return nil
		}

		val = p.Vars.GetFrom(varName, p.Vars.Environment)
	} else {
		if !p.Vars.IsDefined(varName) {
			io.PrintErrf("%q is not defined\n", varName)
			return nil
		}

		val = p.Vars.Get(varName)
	}

	io.Println(val)

	return nil
}

func invokeVarDelete(_ cmdio.IO, varName string, opts varOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// are we looking to delete from a specific environment?
	if opts.envDefaultOverride { // will never be true as user must specify --all to get this behavior
		return fmt.Errorf("--default option cannot be set when deleting a variable")
	}

	// is the 'no default in --env' rule being bypassed by doing --current? reject if we are in the default env
	if opts.envCurrentOverride && p.Vars.Environment == "" {
		return fmt.Errorf("--current option not supported for deletion from default env")
	}

	if opts.envAll {
		// easy, just delete from all environments
		p.Vars.Remove(varName)
		return p.PersistToDisk(false)
	}

	if opts.envOverride != "" && !strings.EqualFold(p.Vars.Environment, strings.ToUpper(opts.envOverride)) {
		p.Vars.UnsetIn(opts.envOverride, varName)
	} else if opts.envCurrentOverride {
		p.Vars.UnsetIn(p.Vars.Environment, varName)
	} else {
		p.Vars.Unset(varName)
	}

	return p.PersistToDisk(false)
}

func invokeVarList(io cmdio.IO, opts varOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	var vars []string
	// are we looking to get from a specific environment?
	if opts.envOverride != "" || opts.envDefaultOverride || opts.envCurrentOverride {
		// we want a specific environment only.

		// either envDefaultOverride is set, meaning we should use the default,
		// so envOverride will be empty. Or, envOverride will never be empty if
		// envDefaultOverride is not set.
		targetEnv := opts.envOverride

		// ...unless we have a default env override set, in which case targetEnv
		// is simply the current one.")
		if opts.envCurrentOverride {
			targetEnv = p.Vars.Environment
		}
		vars = p.Vars.DefinedIn(targetEnv)
	} else {
		vars = p.Vars.All()
	}

	// alphabetize the vars
	sort.Strings(vars)

	if len(vars) == 0 {
		io.Println("(none)")
	} else {
		for _, name := range vars {
			io.Printf("${%s} = %q\n", name, p.Vars.Get(name))
		}
	}

	return nil
}
