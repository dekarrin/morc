package commands

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use: "init [PROJECT_NAME]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "init [PROJECT_NAME]",
	},
	GroupID: "project",
	Short:   "Initialize a new MORC project in the current directory.",
	Long: "Initialize a new MORC project with a project file, session file, and history file located in .morc in " +
		"the current directory. For control over file locations and other initial settings, use 'morc proj new' " +
		"instead.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projName := "Unnamed Project"
		if len(args) > 0 {
			projName = args[0]
		}

		if projName == "" {
			return fmt.Errorf("project name cannot be set to empty")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		io.Quiet = flags.BQuiet

		return invokeInit(io, projName)
	},
}

func init() {
	initCmd.PersistentFlags().BoolVarP(&flags.BQuiet, "quiet", "q", false, "Suppress all unnecessary output.")

	rootCmd.AddCommand(initCmd)
}

func invokeInit(io cmdio.IO, projName string) error {
	p := morc.Project{
		Name:      projName,
		Templates: map[string]morc.RequestTemplate{},
		Flows:     map[string]morc.Flow{},
		Vars:      morc.NewVarStore(),
		History:   []morc.HistoryEntry{},
		Session:   morc.Session{},
		Config: morc.Settings{
			CookieLifetime: 24 * time.Hour,
			ProjFile:       morc.DefaultProjectPath,
			HistFile:       morc.DefaultHistoryPath,
			SeshFile:       morc.DefaultSessionPath,
			RecordHistory:  true,
			RecordSession:  true,
			VarPrefix:      "$",
		},
	}

	// actually do a check as to the existence of prior files before writing
	if _, err := os.Lstat(p.Config.ProjFile); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("init would overwrite an existing project file; remove it first")
	}

	if _, err := os.Lstat(p.Config.SessionFSPath()); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("init would overwrite an existing session file; remove it first")
	}

	if _, err := os.Lstat(p.Config.HistoryFSPath()); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("init would overwrite an existing history file; remove it first")
	}

	io.PrintLoudf("Project created successfully in %s\n", p.Config.ProjFile)

	return writeProject(p, true)
}
