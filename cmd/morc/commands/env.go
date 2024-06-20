package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use: "env [ENV]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"env [--all]\n" +
			"env [ENV | --default]\n" +
			"env [--delete ENV | --delete-all]",
	},
	GroupID: "project",
	Short:   "Show or manipulate request variable environments",
	Long: "With no other arguments, prints out the current variable environment's name. If in the default " +
		"environment, this will be \"" + reservedDefaultEnvName + "\". If given --all, lists all environments. If ENV " +
		"is given, the environment is switched to that one. The default env cannot be selected this way; to specify a " +
		"swap to the default one, use the --default flag instead of giving a name.\n\n" +
		"If -D is given with the name of an environment, the environment is deleted, which clears all variables in " +
		"that environment. Doing so in the default environment would have the effect of clearning every single " +
		"variable across all environments, so to avoid accidental erasure this operation cannot be done by specifying " +
		"--default or --all. Instead, to clear all environments (and therefore all variables across all " +
		"environments), use --delete-all.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args envArgs
		if err := parseEnvArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch args.action {
		case envActionList:
			return invokeEnvList(io, args.projFile)
		case envActionDelete:
			return invokeEnvDelete(io, args.projFile, args.env)
		case envActionSwitch:
			return invokeEnvSwitch(io, args.projFile, args.env)
		case envActionShow:
			return invokeEnvShowCurrent(io, args.projFile)
		default:
			return fmt.Errorf("unhandled env action %d", args.action)
		}
	},
}

func init() {
	envCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	envCmd.PersistentFlags().StringVarP(&flags.Delete, "delete", "D", "", "Delete environment `ENV`")
	envCmd.PersistentFlags().BoolVarP(&flags.BDeleteAll, "delete-all", "", false, "Delete all environments and variables")
	envCmd.PersistentFlags().BoolVarP(&flags.BAll, "all", "a", false, "List all environments instead of only the current one")
	envCmd.PersistentFlags().BoolVarP(&flags.BDefault, "default", "", false, "Change to the default environment")

	// mark the delete and default flags as mutually exclusive
	envCmd.MarkFlagsMutuallyExclusive("all", "default", "delete", "delete-all")

	rootCmd.AddCommand(envCmd)
}

func invokeEnvList(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
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

func invokeEnvDelete(io cmdio.IO, projFile string, env envSelection) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if env.useAll {
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
	} else if env.useName != "" {
		// delete in the specified environment

		allVars := p.Vars.DefinedIn(env.useName)

		for _, varName := range allVars {
			p.Vars.UnsetIn(varName, env.useName)
		}

		p.Vars.DeleteEnv(env.useName)
	} else {
		panic("neither useAll nor useName set; should never happen")
	}

	if err := writeProject(p, false); err != nil {
		return err
	}

	if env.useAll {
		io.PrintLoudf("Deleted all environments and variables")
	} else {
		io.PrintLoudf("Deleted environment %q", env.useName)
	}

	return nil
}

func invokeEnvSwitch(io cmdio.IO, projFile string, env envSelection) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// caller already has set env to "" if we are going to the default env
	p.Vars.Environment = env.useName

	if err := writeProject(p, false); err != nil {
		return err
	}

	io.PrintLoudf("Switched to %s", env.String())
	return nil
}

func invokeEnvShowCurrent(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
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

type envArgs struct {
	projFile string
	action   envAction
	env      envSelection
}

func parseEnvArgs(cmd *cobra.Command, posArgs []string, args *envArgs) error {
	args.projFile = flags.ProjectFile
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseEnvActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	// do action-specific arg and flag parsing
	f := cmd.Flags()
	switch args.action {
	case envActionList:
		// nothing to do here; only --all gets us here
	case envActionDelete:
		// both --delete-all and --delete will get us here. If it's --delete,
		// that's the thing to grab.
		if f.Changed("delete") {
			args.env.useName = flags.Delete
		} else {
			args.env.useAll = true
		}
	case envActionSwitch:
		// both --default and an arg will get us here.
		if f.Changed("default") {
			args.env.useDefault = true
		} else {
			args.env.useName = posArgs[0]
		}
	case envActionShow:
		// nothing else to grab
	default:
		panic(fmt.Sprintf("unhandled vars action %q", args.action))
	}

	return nil
}

func parseEnvActionFromFlags(cmd *cobra.Command, posArgs []string) (envAction, error) {
	f := cmd.Flags()

	if f.Changed("delete") {
		if len(posArgs) > 1 {
			return envActionDelete, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}

		if flags.Delete == reservedDefaultEnvName {
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

type envAction int

const (
	envActionList envAction = iota
	envActionDelete
	envActionSwitch
	envActionShow
)
