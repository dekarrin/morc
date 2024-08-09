package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/internal/sliceops"
	"github.com/dekarrin/rosed"
	"github.com/spf13/cobra"
)

var varsCmd = &cobra.Command{
	Use: "vars [VAR [VALUE]]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"vars [-env ENV | --current | --default]\n" +
			"vars --delete VAR [-env ENV | --current | --default | --all]\n" +
			"vars VAR [-env ENV | --current | --default | --all]\n" +
			"vars VAR VALUE [-env ENV | --current | --default | --all]",
	},
	GroupID: "project",
	Short:   "Show or manipulate request variables",
	Long: "Without any other arguments, vars prints a listing of the variables accessible from the current variable " +
		"environment, including any values filled from the default environment. --env=ENV can be passed in to show only " +
		"the values defined in the given environment ENV, or as a shortcut, --current can be used to specify the current " +
		"environment. To see only the default variable values, use --default.\n\n" +
		"Variables are created by specifying both the name of a variable, VAR, and a VALUE for the variable as arguments. " +
		"This will set the value of the variable in the current environment. If the current environment is not the default, " +
		"the new var will be created there as well (with a blank value) if it does not already exist. --env=ENV, --current, " +
		"and --default may all be used to set a variable in an environment other than the current one (or explicitly set " +
		"it in the current one in the case of --current). If --all is given, the variable will have its value set in every " +
		"existing environment, including the default one.\n\n" +
		"To get the value of a variable, pass the name of the variable, VAR, as an argument without specifying a new value " +
		"for it. --env=ENV, --current, and --default can be used to specify the environment to get the value from. When " +
		"getting a var, --current and --env=ENV will only retrieve the value if the var is defined in that environment. " +
		"--all will give a listing of all values of VAR across all environments, including the default one.\n\n" +
		"A variable is deleted by passing the flag --delete with the name of the VAR as an argument to it. Similarly to the " +
		"other commands, --env, --current, --default, and --all can be used to specify deletion from an environment other " +
		"than the current one.",
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
	varsCmd.PersistentFlags().StringVarP(&flags.Env, "env", "e", "", "Apply to environment `ENV` instead of the current one. Use --default instead to specify the default environment.")
	varsCmd.PersistentFlags().BoolVarP(&flags.BDefault, "default", "", false, "Apply to the default environment.")
	varsCmd.PersistentFlags().BoolVarP(&flags.BCurrent, "current", "", false, "Apply only to current environment. This is the same as --env followed by the name of the current environment.")
	varsCmd.PersistentFlags().BoolVarP(&flags.BAll, "all", "a", false, "Apply to all environments. The meaning varies based on the operation being performed. When deleting, this will delete the variable from all environments. When getting, this will list all values of the variable in each env that defines it. When setting, it sets the value of the variable in all environments to the given value.")

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

	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if env.useAll {
		// get list of envs to set in
		allEnvs := p.Vars.EnvNames()
		for _, envName := range allEnvs {
			p.Vars.SetIn(varName, value, envName)
		}
		io.PrintLoudf("Set %s{%s} to %q in all envs\n", p.VarPrefix(), varName, value)
	} else if env.useDefault {
		p.Vars.SetIn(varName, value, "")
		io.PrintLoudf("Set %s{%s} to %q in default env\n", p.VarPrefix(), varName, value)
	} else if env.useName != "" {
		p.Vars.SetIn(varName, value, env.useName)
		io.PrintLoudf("Set %s{%s} to %q in env %s\n", p.VarPrefix(), varName, value, env.useName)
	} else if env.useCurrent {
		p.Vars.SetIn(varName, value, p.Vars.Environment)
		io.PrintLoudf("Set %s{%s} to %q in current env\n", p.VarPrefix(), varName, value)
	} else {
		p.Vars.Set(varName, value)
		io.PrintLoudf("Set %s{%s} to %q\n", p.VarPrefix(), varName, value)
	}

	return writeProject(p, false)
}

func invokeVarGet(io cmdio.IO, projFile string, env envSelection, varName string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	var val string
	if env.useAll {
		// NOTE: this will short-circ out of the if block because it produces
		// its own special output. It will not continue to the return at the
		// the bottom.

		nonDefs := p.Vars.NonDefaultEnvsWith(varName)
		sort.Strings(nonDefs)
		inEnvs := nonDefs

		if p.Vars.IsDefinedIn(varName, "") {
			inEnvs = make([]string, len(nonDefs)+1)
			inEnvs[0] = ""
			copy(inEnvs[1:], nonDefs)
		}

		if len(inEnvs) == 0 {
			io.PrintErrf("(no values defined)\n")
			return nil
		}

		tableData := [][]string{}

		doHeaders := !io.Quiet
		if doHeaders {
			tableData = append(tableData, []string{"Env", "Value"})
		}

		for _, envName := range inEnvs {
			val = p.Vars.GetFrom(varName, envName)
			displayName := strings.ToUpper(envName)
			if envName == "" {
				displayName = "(default)"
			}

			if !io.Quiet {
				displayName += ":"
			}

			displayVal := val
			if !io.Quiet {
				displayVal = fmt.Sprintf("%q", val)
			}

			tableData = append(tableData, []string{displayName, displayVal})
		}

		const minWidth = 8
		output := rosed.Editor{}.
			InsertTableOpts(
				rosed.End,
				tableData,
				minWidth,
				rosed.Options{
					TableHeaders: doHeaders,
				},
			).
			String()

		io.Printf("%s", output)
		return nil
	} else if env.useDefault {
		if !p.Vars.IsDefinedIn(varName, "") {
			io.PrintErrf("%s{%s} is not defined in default env\n", p.VarPrefix(), varName)
			return nil
		}

		val = p.Vars.GetFrom(varName, "")
	} else if env.useName != "" {
		if !p.Vars.IsDefinedIn(varName, env.useName) {
			// does it have a value via default? say so if so
			valueViaDefault := ""
			if p.Vars.IsDefined(varName) {
				valueViaDefault = "; value is via default env"
			}

			io.PrintErrf("%s{%s} is not defined in env %s%s\n", p.VarPrefix(), varName, strings.ToUpper(env.useName), valueViaDefault)
			return nil
		}

		val = p.Vars.GetFrom(varName, env.useName)
	} else if env.useCurrent {
		if !p.Vars.IsDefinedIn(varName, p.Vars.Environment) {
			var envName = strings.ToUpper(p.Vars.Environment)
			if envName == "" {
				envName = "default env"
			}

			// does it have a value via default? say so if so
			valueViaDefault := ""
			if p.Vars.IsDefined(varName) {
				valueViaDefault = "; value is via default env"
			}

			io.PrintErrf("%s{%s} is not defined in current env (%s)%s\n", p.VarPrefix(), varName, envName, valueViaDefault)
			return nil
		}

		val = p.Vars.GetFrom(varName, p.Vars.Environment)
	} else {
		if !p.Vars.IsDefined(varName) {
			io.PrintErrf("%s{%s} is not defined\n", p.VarPrefix(), varName)
			return nil
		}

		val = p.Vars.Get(varName)
	}

	io.Println(val)

	return nil
}

// TODO: standardize env vs environment in output.
// TODO: once unit tests are in place, refactor this whole damn func, glub. it's
// incredibly difficult to follow. Change to use an if-case for each of the env
// selection possibilities and key off of that.
func invokeVarDelete(io cmdio.IO, projFile string, env envSelection, varName string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// are we looking to delete from a specific environment?

	// is the 'no default in --env' rule being bypassed by doing --current? reject if var is present in any other env
	if env.useCurrent && p.Vars.Environment == "" {
		otherEnvs := p.Vars.NonDefaultEnvsWith(varName)
		if len(otherEnvs) > 0 {
			sort.Strings(otherEnvs)
			return fmt.Errorf("cannot remove %s{%s} from current env (default env)\nValue is also defined in envs: %s\nSet --all to delete from all environments", p.VarPrefix(), varName, strings.Join(otherEnvs, ", "))
		}
	}

	if env.useAll {
		// easy, just delete from all environments
		if !p.Vars.IsDefined(varName) {
			return fmt.Errorf("%s{%s} does not exist in any environment", p.VarPrefix(), varName)
		}

		p.Vars.Remove(varName)
		if err := writeProject(p, false); err != nil {
			return err
		}

		io.PrintLoudf("Deleted %s{%s} from all environments\n", p.VarPrefix(), varName)
		return nil
	}

	// is the user currently in the default environment AND not specifying an
	// env AND at least one other
	// env with the to-be-deleted var is defined? if so, opts.envAll is required
	// and they should have provided that if this is what they really want
	if !env.IsSpecified() && p.Vars.Environment == "" {
		otherEnvs := p.Vars.NonDefaultEnvsWith(varName)

		if len(otherEnvs) > 0 {
			sort.Strings(otherEnvs)
			return fmt.Errorf("%s{%s} is also defined in non-default envs: %s\nSet --all to delete from all environments", p.VarPrefix(), varName, strings.Join(otherEnvs, ", "))
		}
	}

	if env.useDefault {
		if !p.Vars.IsDefinedIn(varName, "") {
			return fmt.Errorf("%s{%s} does not exist in default env", p.VarPrefix(), varName)
		}

		// otherwise, we can delete ONLY if the var is not defined in any other env
		nonDefaultEnvs := p.Vars.NonDefaultEnvsWith(varName)
		if len(nonDefaultEnvs) > 0 {
			sort.Strings(nonDefaultEnvs)
			return fmt.Errorf("cannot remove %s{%s} from default env\nValue is also defined in envs: %s\nSet --all to delete from all environments", p.VarPrefix(), varName, strings.Join(nonDefaultEnvs, ", "))
		}

		p.Vars.Remove(varName)
	} else if env.useName != "" {
		if !p.Vars.IsDefinedIn(varName, env.useName) {
			// if it is not defined in the given env, we are not going to delete it
			// but we will perform some checks to give better error reporting

			// if it exists in default only we will not delete
			// bc user explicitly asked for deletion from specific one only glub
			if p.Vars.IsDefinedIn(varName, "") {
				return fmt.Errorf("%s{%s} is not defined in env %s; value is via default env", p.VarPrefix(), varName, env.useName)
			}

			return fmt.Errorf("%s{%s} does not exist in env %s", p.VarPrefix(), varName, env.useName)
		}

		p.Vars.UnsetIn(varName, env.useName)
	} else if env.useCurrent {
		if !p.Vars.IsDefinedIn(varName, p.Vars.Environment) {

			// if it exists in default only and not in current, we will not delete
			// bc user explicitly asked for deletion from current only
			if p.Vars.IsDefinedIn(varName, "") {
				return fmt.Errorf("%s{%s} is not defined in current env; value is via default env", p.VarPrefix(), varName)
			}

			return fmt.Errorf("%s{%s} does not exist in current environment", p.VarPrefix(), varName)
		}

		p.Vars.UnsetIn(varName, p.Vars.Environment)
	} else {
		if !p.Vars.IsDefined(varName) {
			return fmt.Errorf("%s{%s} does not exist", p.VarPrefix(), varName)
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
				sort.Strings(otherEnvs)
				return fmt.Errorf("cannot remove %s{%s}\nValue is via default env and var is defined in envs: %s\nSet --all to delete from all environments", p.VarPrefix(), varName, strings.Join(otherEnvs, ", "))
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

	if err := writeProject(p, false); err != nil {
		return err
	}

	fromMsg := ""
	if env.IsSpecified() {
		fromMsg = fmt.Sprintf(" from %s", env)
	}
	io.PrintLoudf("Deleted %s{%s}%s\n", p.VarPrefix(), varName, fromMsg)
	return nil
}

func invokeVarList(io cmdio.IO, projFile string, env envSelection) error {
	p, err := readProject(projFile, true)
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
			io.Printf("%s{%s} = %q\n", p.VarPrefix(), name, v)
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
	args.projFile = projPathFromFlagsOrFile(cmd)
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
		args.env.useAll = flags.BAll
	case varsActionSet:
		args.varName = posArgs[0]
		args.value = posArgs[1]
		args.env.useAll = flags.BAll
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

		if flags.Env == reservedDefaultEnvName {
			return varsActionDelete, fmt.Errorf("cannot specify reserved env name %q; use --default or --all to specify the default env", reservedDefaultEnvName)
		}
		if f.Changed("env") && flags.Env == "" {
			return varsActionDelete, fmt.Errorf("cannot specify env \"\"; use --default or --all to specify the default env")
		}
		return varsActionDelete, nil
	}

	if len(posArgs) == 0 {
		// listing mode
		if flags.BAll {
			return varsActionList, fmt.Errorf("--all has no effect when listing vars; use --default to list vars in default env")
		}
		if flags.Env == reservedDefaultEnvName {
			return varsActionList, fmt.Errorf("cannot use reserved environment name %q; use --default to list vars in default env", reservedDefaultEnvName)
		}
		if f.Changed("env") && flags.Env == "" {
			return varsActionDelete, fmt.Errorf("cannot specify env \"\"; use --default to list vars in default env")
		}
		return varsActionList, nil
	} else if len(posArgs) == 1 {
		// getting mode

		if flags.Env == reservedDefaultEnvName {
			return varsActionGet, fmt.Errorf("cannot specify reserved env name %q; use --default to get from default env", reservedDefaultEnvName)
		}
		if f.Changed("env") && flags.Env == "" {
			return varsActionGet, fmt.Errorf("cannot specify env \"\"; use --default to get from default env")
		}
		return varsActionGet, nil
	} else if len(posArgs) == 2 {
		// setting mode
		if flags.Env == reservedDefaultEnvName {
			return varsActionGet, fmt.Errorf("cannot specify reserved env name %q; use --default to set in default env", reservedDefaultEnvName)
		}
		if f.Changed("env") && flags.Env == "" {
			return varsActionGet, fmt.Errorf("cannot specify env \"\"; use --default to set in default env")
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
