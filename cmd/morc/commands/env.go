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
	flagEnvDelete      bool
	flagEnvAll         bool
	flagEnvDefault     bool
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
	envCmd.PersistentFlags().BoolVarP(&flagEnvDelete, "delete", "d", false, "Delete the specified environment")
	envCmd.PersistentFlags().BoolVarP(&flagEnvAll, "all", "a", false, "List all environments when used without -d. Delete all environments and variables when used with -d")
	envCmd.PersistentFlags().BoolVarP(&flagEnvDefault, "default", "", false, "Change to the default environment")

	// mark the delete and default flags as mutually exclusive
	envCmd.MarkFlagsMutuallyExclusive("all", "default")

	rootCmd.AddCommand(envCmd)
}

var envCmd = &cobra.Command{
	Use:     "env [NAME] [-F project_file] [-d] [--all] [--default]",
	GroupID: "project",
	Short:   "Show or manipulate request variable environments",
	Long:    "With no other arguments, prints out the current variable environment's name. If in the default environment, this will be \"" + reservedDefaultEnvName + "\". If given --all, lists all environments. If NAME is given with no other flags, the environment is switched to that one. The default env cannot be selected with NAME; to specify to the default one, use the --default flag instead of giving a name.\n\nIf -d is given with NAME, the environment is deleted, which clears all variables in that environment. Since doing so in the default environment has the effect of clearning every single variable across all environments, this operation cannot be done by specifying --default to avoid accidental erasure. Instead, to clear all variables across all environments, -d can be used with --all instead of NAME to explicitly declare intent.",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := envOptions{
			projFile:      flagEnvProjectFile,
			doAll:         flagEnvAll,
			doDelete:      flagEnvDelete,
			swapToDefault: flagEnvDefault,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// depending on mode, actions are: print the current environment,
		// list all environments, switch to a new environment, or delete an
		// environment
		action := envActionShow
		var env string

		if len(args) == 0 {
			if opts.doDelete {
				if opts.doAll {
					action = envActionDelete
				}

				// error modes:
				if opts.swapToDefault {
					return fmt.Errorf("cannot use --default with --delete; use --all with --delete instead to confirm intent")
				}
				return fmt.Errorf("must specify environment to delete")
			} else if opts.swapToDefault {
				action = envActionSwitch
			} else if opts.doAll {
				action = envActionList
			}

			// otherwise, leave action as just print the current environment
		} else {
			env = args[0]

			if env == "" {
				if opts.doDelete {
					return fmt.Errorf("environment name cannot be empty; use --all to confirm deletion from default env and therefore all others")
				}

				return fmt.Errorf("environment name cannot be empty; use --default instead to swap to the default environment")
			}

			if opts.doAll {
				return fmt.Errorf("--all cannot be used with an environment name")
			}

			if opts.swapToDefault {
				return fmt.Errorf("--default cannot be used with an environment name")
			}

			if opts.doDelete {
				if args[0] == reservedDefaultEnvName {
					return fmt.Errorf("refusing to delete the default environment as this clears all vars; use -d --all with no other args to confirm intent")
				}
				action = envActionDelete
			} else {

				if args[0] == reservedDefaultEnvName {
					return fmt.Errorf("do not specify default env by name; use --default instead")
				}
				action = envActionSwitch
			}
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
	} else {
		// delete in the specified environment

		allVars := p.Vars.DefinedIn(env)

		for _, varName := range allVars {
			p.Vars.UnsetIn(varName, env)
		}
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
