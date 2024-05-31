package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var (
	flagEnvProjectFile string
	flagEnvDelete      string
	flagEnvAll         bool
	flagEnvDefault     bool
	flagEnvDeleteAll   bool
)

type envAction int

const (
	envActionList envAction = iota
	envActionDelete
	envActionSwitch
	envActionShow
)

func init() {
	envCmd.PersistentFlags().StringVarP(&flagEnvProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	envCmd.PersistentFlags().StringVarP(&flagEnvDelete, "delete", "D", "", "Delete environment `ENV`")
	envCmd.PersistentFlags().BoolVarP(&flagEnvDeleteAll, "delete-all", "", false, "Delete all environments and variables")
	envCmd.PersistentFlags().BoolVarP(&flagEnvAll, "all", "a", false, "List all environments instead of only the current one")
	envCmd.PersistentFlags().BoolVarP(&flagEnvDefault, "default", "", false, "Change to the default environment")

	// mark the delete and default flags as mutually exclusive
	envCmd.MarkFlagsMutuallyExclusive("all", "default", "delete", "delete-all")

	rootCmd.AddCommand(envCmd)
}

// env --all - LIST envs -- allows flags
// env NAME|--default - SWITCH env - allows NO flags
// env - SHOW
// env [-D NAME]|[--delete-all] - delete env NAME or all.

var envCmd = &cobra.Command{
	Use: "env [-F FILE]\n" +
		"env [-F FILE] --all\n" +
		"env [-F FILE] ENV\n" +
		"env [-F FILE] --default\n" +
		"env [-F FILE] -D ENV\n" +
		"env [-F FILE] --delete-all",
	GroupID: "project",
	Short:   "Show or manipulate request variable environments",
	Long:    "With no other arguments, prints out the current variable environment's name. If in the default environment, this will be \"" + reservedDefaultEnvName + "\". If given --all, lists all environments. If NAME is given, the environment is switched to that one. The default env cannot be selected this way; to specify a swap to the default one, use the --default flag instead of giving a name.\n\nIf -D is given with the name of an environment, the environment is deleted, which clears all variables in that environment. Since doing so in the default environment has the effect of clearning every single variable across all environments, this operation cannot be done by specifying --default or --all to avoid accidental erasure. Instead, to clear all variables across all environments, use --delete-all",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := envOptions{
			projFile:      flagEnvProjectFile,
			doAll:         flagEnvAll, // TODO: during refactor, eliminate this. it's overloaded.
			doDelete:      flagEnvDelete != "",
			swapToDefault: flagEnvDefault,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// TOOD: refactor arg parsing to match pattern in reqs, proj, flows, caps.

		// depending on mode, actions are: print the current environment,
		// list all environments, switch to a new environment, or delete an
		// environment
		action, err := parseEnvActionFromFlags(cmd, args)
		if err != nil {
			return err
		}

		var env string
		f := cmd.Flags()

		// pick up args based on action
		switch action {
		case envActionList:
			// nothing else to grab; only --all does this.
		case envActionDelete:
			// both --delete-all and --delete will get us here. If it's --delete,
			// that's the ting to grab.
			if f.Changed("delete") {
				env = flagEnvDelete
			} else {
				opts.doAll = true
			}
		case envActionSwitch:
			// both --default and an arg will get us here. If it's --default, that's
			// already set from above.
			if f.Changed("default") {
				opts.swapToDefault = true
			} else {
				env = args[0]
			}
		case envActionShow:
			// nothing else to grab
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch action {
		case envActionList:
			return invokeEnvList(io, opts)
		case envActionDelete:
			return invokeEnvDelete(io, env, opts)
		case envActionSwitch:
			return invokeEnvSwitch(io, env, opts)
		case envActionShow:
			return invokeEnvShowCurrent(io, opts)
		default:
			return fmt.Errorf("unhandled action %d", action)
		}
	},
}

type envOptions struct {
	projFile      string
	doAll         bool
	doDelete      bool
	swapToDefault bool
}

func invokeEnvList(io cmdio.IO, opts envOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	envs := p.Vars.EnvNames()

	// if current env isn't in there, add it
	var inEnvs bool
	for _, env := range envs {
		if env == strings.ToUpper(p.Vars.Environment) {
			inEnvs = true
			break
		}
	}

	if !inEnvs {
		envs = append(envs, strings.ToUpper(p.Vars.Environment))
	}

	// alphabetize it
	sort.Strings(envs)

	for _, env := range envs {
		if env == "" {
			env = reservedDefaultEnvName
		}
		io.Println(env)
	}

	return nil
}

func invokeEnvDelete(_ cmdio.IO, env string, opts envOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if opts.doAll {
		allVars := p.Vars.All()

		for _, varName := range allVars {
			p.Vars.Remove(varName)
		}

		// clear the environments
		for _, envName := range p.Vars.EnvNames() {
			if envName == "" {
				continue
			}
			p.Vars.DeleteEnv(envName)
		}
	} else {
		// delete in the specified environment

		allVars := p.Vars.DefinedIn(env)

		for _, varName := range allVars {
			p.Vars.UnsetIn(varName, env)
		}

		p.Vars.DeleteEnv(env)
	}

	return p.PersistToDisk(false)
}

func invokeEnvSwitch(_ cmdio.IO, env string, opts envOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// caller already has set env to "" if we are going to the default env
	p.Vars.Environment = env

	return p.PersistToDisk(false)
}

func invokeEnvShowCurrent(io cmdio.IO, opts envOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if p.Vars.Environment == "" {
		io.Println(reservedDefaultEnvName)
	} else {
		io.Println(strings.ToUpper(p.Vars.Environment))
	}

	return nil
}

func parseEnvActionFromFlags(cmd *cobra.Command, posArgs []string) (envAction, error) {
	f := cmd.Flags()

	if f.Changed("delete") {
		if len(posArgs) > 1 {
			return envActionDelete, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}

		if flagEnvDelete == reservedDefaultEnvName {
			return envActionDelete, fmt.Errorf("cannot use reserved environment name %q; use --delete-all to delete all envs (including default)", reservedDefaultEnvName)
		}
		return envActionDelete, nil
	} else if f.Changed("delete-all") {
		if len(posArgs) > 0 {
			return envActionDelete, fmt.Errorf("unknown positional argument %q", posArgs[0])
		}

		return envActionDelete, nil
	} else if f.Changed("all") {
		if len(posArgs) > 0 {
			return envActionList, fmt.Errorf("unknown positional argument %q", posArgs[0])
		}

		return envActionList, nil
	} else if f.Changed("default") {
		if len(posArgs) > 0 {
			return envActionSwitch, fmt.Errorf("unknown positional argument %q", posArgs[0])
		}

		return envActionSwitch, nil
	}

	if len(posArgs) == 0 {
		// already checked for all, so this is a show
		return envActionShow, nil
	} else if len(posArgs) == 1 {
		// switch
		return envActionSwitch, nil
	}

	return envActionShow, fmt.Errorf("unknown positional argument %q", posArgs[2])
}
