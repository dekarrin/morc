package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/internal/sliceops"
	"github.com/spf13/cobra"
)

var varsCmd = &cobra.Command{
	Use: "vars [VAR [VALUE]]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"vars [-e ENV | --current | --default]\n" +
			"vars --delete VAR [-e ENV | --current | --all]\n" +
			"vars VAR [-e ENV | --current | --default]\n" +
			"vars VAR VALUE [-e ENV | --current | --default]",
	},
	GroupID: "project",
	Short:   "Show or manipulate request variables",
	Long: "Prints out a listing of the variables accessible from the current variable environment (which includes " +
		"any from default environment, not specifically set in current, unless --current or --env or --default is " +
		"given) if given no other arguments. If given the name VAR of a variable, that variable's value will be " +
		"printed out. If given VAR and a VALUE, sets the variable to that value. To delete a variable, pass -D with " +
		"the name VAR of the variable to delete.\n\n" +
		"If --env or --default is used, a listing will exclusively show " +
		"variables defined in that environment, whereas typically it would show values in the current environment, " +
		"supplemented with those from the default environment for vars that are not defined in the specific one. If " +
		"the current environment *is* the default environment, there is no distinction.",
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args varsArgs
		if err := parseVarsArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch args.action {
		case varsActionList:
			return invokeVarList(io, args.projFile, args.env)
		case varsActionGet:
			return invokeVarGet(io, args.projFile, args.env, args.varName)
		case varsActionSet:
			return invokeVarSet(io, args.projFile, args.env, args.varName, args.value)
		case varsActionDelete:
			return invokeVarDelete(io, args.projFile, args.env, args.varName)
		default:
			panic(fmt.Sprintf("unhandled vars action %q", args.action))
		}
	},
}

func init() {
	varsCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath)
	varsCmd.PersistentFlags().StringVarP(&flags.Delete, "delete", "D", "", "Delete the variable `VAR`")
	varsCmd.PersistentFlags().StringVarP(&flags.Env, "env", "e", "", "Run the command against the environment `ENV` instead of the current one. Use --default instead to specify the default environment.")
	varsCmd.PersistentFlags().BoolVarP(&flags.BDefault, "default", "", false, "Run the command against the default environment instead of the current one.")
	varsCmd.PersistentFlags().BoolVarP(&flags.BCurrent, "current", "", false, "Apply only to current environment. This is the same as typing --env followed by the name of the current environment.")
	varsCmd.PersistentFlags().BoolVarP(&flags.BAll, "all", "a", false, "Used with -D. Delete the variable from all environments. This is the only way to effectively specify '--default' while deleting; it is a separate flag to indicate that the variable will indeed be erased everywhere, not just in the default environment.")

	// mark the env and default flags as mutually exclusive
	varsCmd.MarkFlagsMutuallyExclusive("env", "default", "all", "current")

	rootCmd.AddCommand(varsCmd)
}

func invokeVarSet(io cmdio.IO, projFile string, env envSelection, varName, value string) error {
	// dont even bother to load if the var name is invalid
	varName, err := morc.ParseVarName(strings.ToUpper(varName))
	if err != nil {
		return err
	}

	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	if env.useDefault {
		p.Vars.SetIn(varName, value, "")
		io.PrintLoudf("Set ${%s} to %q in default environment\n", varName, value)
	} else if env.useName != "" {
		p.Vars.SetIn(varName, value, env.useName)
		io.PrintLoudf("Set ${%s} to %q in environment %q\n", varName, value, env.useName)
	} else if env.useCurrent {
		p.Vars.SetIn(varName, value, p.Vars.Environment)
		io.PrintLoudf("Set ${%s} to %q in current environment\n", varName, value)
	} else {
		p.Vars.Set(varName, value)
		io.PrintLoudf("Set ${%s} to %q\n", varName, value)
	}

	return p.PersistToDisk(false)
}

func invokeVarGet(io cmdio.IO, projFile string, env envSelection, varName string) error {
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	var val string
	if env.useDefault {
		if !p.Vars.IsDefinedIn(varName, "") {
			io.PrintErrf("%q is not defined in default environment\n", varName)
			return nil
		}

		val = p.Vars.GetFrom(varName, "")
	} else if env.useName != "" {
		if !p.Vars.IsDefinedIn(varName, env.useName) {
			io.PrintErrf("%q is not defined in environment %q\n", varName, env.useName)
			return nil
		}

		val = p.Vars.GetFrom(varName, env.useName)
	} else if env.useCurrent {
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

func invokeVarDelete(io cmdio.IO, projFile string, env envSelection, varName string) error {
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	// are we looking to delete from a specific environment?
	if env.useDefault {
		// should never happen as user must specify --all to get this behavior
		return fmt.Errorf("cannot delete from default environment by specifying useDefault; set all=true instead")
	}

	// is the 'no default in --env' rule being bypassed by doing --current? reject if we are in the default env
	if env.useCurrent && p.Vars.Environment == "" {
		return fmt.Errorf("cannot delete from current env when current env is default env; set all=true instead")
	}

	if env.useAll {
		// easy, just delete from all environments
		if !p.Vars.IsDefined(varName) {
			return fmt.Errorf("${%s} does not exist in any environment", varName)
		}

		p.Vars.Remove(varName)
		if err := p.PersistToDisk(false); err != nil {
			return err
		}

		io.PrintLoudf("Deleted ${%s} from all environments\n", varName)
		return nil
	}

	// is the user currently in the default environment *and* at least one other
	// env with the to-be-deleted var is defined? if so, opts.envAll is required
	// and they should have provided that if this is what they really want
	if p.Vars.Environment == "" {
		otherEnvs := p.Vars.NonDefaultEnvsWith(varName)

		if len(otherEnvs) > 0 {
			return fmt.Errorf("${%s} is also defined in non-default envs: %s\nSet --all to delete from all environments", varName, strings.Join(otherEnvs, ", "))
		}
	}

	if env.useName != "" && !strings.EqualFold(p.Vars.Environment, env.useName) {
		// it is some env other than the current one
		if !p.Vars.IsDefinedIn(varName, env.useName) {
			return fmt.Errorf("${%s} does not exist in environment %q", varName, env.useName)
		}
		p.Vars.UnsetIn(env.useName, varName)
	} else if env.useCurrent {
		if !p.Vars.IsDefinedIn(varName, p.Vars.Environment) {
			return fmt.Errorf("${%s} does not exist in current environment", varName)
		}

		// if it exists in default only and not in current, we will not delete
		// bc user explicitly asked for deletion from current only
		if p.Vars.IsDefinedIn(varName, "") && !p.Vars.IsDefinedIn(varName, p.Vars.Environment) {
			return fmt.Errorf("${%s} is not defined in current env %s; value is via default env", varName, strings.ToUpper(p.Vars.Environment))
		}
		p.Vars.UnsetIn(p.Vars.Environment, varName)
	} else {
		if !p.Vars.IsDefined(varName) {
			return fmt.Errorf("${%s} does not exist", varName)
		}

		if p.Vars.IsDefinedIn(varName, "") && !p.Vars.IsDefinedIn(varName, p.Vars.Environment) {
			// it exists in default and not the current env.

			// if it exists in default only and at least one other env, we will not
			// delete unless --all is given
			nonDefaultEnvs := p.Vars.NonDefaultEnvsWith(varName)
			otherEnvs := sliceops.Filter(nonDefaultEnvs, func(s string) bool {
				return !strings.EqualFold(s, p.Vars.Environment)
			})

			if len(otherEnvs) > 0 {
				return fmt.Errorf("cannot remove ${%s}\nValue is via default env and var is defined in envs: %s\nSet --all to delete from all environments", varName, strings.Join(otherEnvs, ", "))
			}

			// default env deletion
			p.Vars.Remove(varName)

			// set env selector so output is correct
			env.useDefault = true
		} else {
			// normal deletion
			p.Vars.Unset(varName)
		}
	}

	if err := p.PersistToDisk(false); err != nil {
		return err
	}

	fromMsg := ""
	if env.IsSpecified() {
		fromMsg = fmt.Sprintf(" from %s", env)
	}
	io.PrintLoudf("Deleted ${%s}%s\n", varName, fromMsg)
	return nil
}

func invokeVarList(io cmdio.IO, projFile string, env envSelection) error {
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	var vars []string

	var targetEnv string

	// are we looking to get from a specific environment?
	if env.IsSpecified() {
		// we want a specific environment only.

		// either env.useDefault is set, meaning we should use the default,
		// so env.useName will be empty. Or, env.useName will never be empty if
		// env.useDefault is not set.
		targetEnv = env.useName

		// ...unless we have env.useCurrnet set, in which case targetEnv
		// is simply the current one.
		if env.useCurrent {
			targetEnv = p.Vars.Environment
		}

		// TODO: bug? we never checked for env.useDefault here. Be shore to
		// cover in tests
		vars = p.Vars.DefinedIn(targetEnv)
	} else {
		vars = p.Vars.All()
	}

	// alphabetize the vars
	sort.Strings(vars)

	if len(vars) == 0 {
		io.Println("(none)")
	} else {
		var v string
		for _, name := range vars {
			if env.IsSpecified() {
				v = p.Vars.GetFrom(name, targetEnv)
			} else {
				v = p.Vars.Get(name)
			}
			io.Printf("${%s} = %q\n", name, v)
		}
	}

	return nil
}

type varsArgs struct {
	projFile string
	action   varsAction
	env      envSelection
	varName  string
	value    string
}

func parseVarsArgs(cmd *cobra.Command, posArgs []string, args *varsArgs) error {
	args.projFile = flags.ProjectFile
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseVarsActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	// all actions allow for environment selection so grab that now
	if flags.Env != "" {
		args.env.useName = flags.Env
	} else if flags.BDefault {
		// --delete doesn't allow this flag, but we already checked in action
		// parsing.
		args.env.useDefault = true
	} else if flags.BCurrent {
		args.env.useCurrent = true
	}

	// do action-specific arg and flag parsing
	switch args.action {
	case varsActionList:
		// nothing to do here
	case varsActionGet:
		args.varName = posArgs[0]
	case varsActionSet:
		args.varName = posArgs[0]
		args.value = posArgs[1]
	case varsActionDelete:
		args.varName = flags.Delete
		args.env.useAll = flags.BAll
	default:
		panic(fmt.Sprintf("unhandled vars action %q", args.action))
	}

	return nil
}

func parseVarsActionFromFlags(cmd *cobra.Command, posArgs []string) (varsAction, error) {
	// mutual exclusions enforced by cobra (and therefore we do not check them here):
	// * --env, --default, --all, --current
	//
	// we do NOT enforce on --delete and --default at the cobra level so we can
	// return a custom error message.

	f := cmd.Flags()

	if f.Changed("delete") {
		if len(posArgs) > 1 {
			return varsActionDelete, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}

		if f.Changed("default") {
			return varsActionDelete, fmt.Errorf("cannot specify --default with --delete/-D; use --all to delete from all envs")
		}
		if flags.Env == reservedDefaultEnvName {
			return varsActionDelete, fmt.Errorf("cannot use reserved environment name %q; use --all to delete from all envs (including default)", reservedDefaultEnvName)
		}
		return varsActionDelete, nil
	}

	if len(posArgs) == 0 {
		// listing mode
		if flags.BAll {
			return varsActionList, fmt.Errorf("--all is only valid when deleting a var; use --default to list vars in default env")
		}
		if flags.Env == reservedDefaultEnvName {
			return varsActionList, fmt.Errorf("cannot use reserved environment name %q; use --default to list vars in default env", reservedDefaultEnvName)
		}
		return varsActionList, nil
	} else if len(posArgs) == 1 {
		// setting mode
		if flags.BAll {
			return varsActionGet, fmt.Errorf("--all is only valid when deleting a var; use --default to get from default env")
		}
		if flags.Env == reservedDefaultEnvName {
			return varsActionList, fmt.Errorf("cannot use reserved environment name %q; use --default to get from default env", reservedDefaultEnvName)
		}
		return varsActionGet, nil
	} else if len(posArgs) == 2 {
		// setting mode
		if flags.BAll {
			return varsActionSet, fmt.Errorf("--all is only valid when deleting; use --default to set var in the default environment")
		}
		if flags.Env == reservedDefaultEnvName {
			return varsActionList, fmt.Errorf("cannot use reserved environment name %q; use --default to set in default env", reservedDefaultEnvName)
		}
		return varsActionSet, nil
	}

	return varsActionList, fmt.Errorf("unknown positional argument %q", posArgs[2])
}

type varsAction int

const (
	varsActionList varsAction = iota
	varsActionGet
	varsActionSet
	varsActionDelete
)
