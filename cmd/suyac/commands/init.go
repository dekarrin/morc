package commands

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:     "init [PROJ_NAME]",
	GroupID: "project",
	Short:   "Initialize a new Suyac project in the current directory.",
	Long:    "Initialize a new Suyac project with a project file, session file, and history file located in .suyac in the current directory. For control over file locations and other initial settings, use 'suyac proj new' instead.",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projName := "Unnamed Project"
		if len(args) > 0 {
			projName = args[0]
		}

		if projName == "" {
			return fmt.Errorf("project name cannot be set to empty")
		}

		return invokeInit(projName)
	},
}

func invokeInit(projName string) error {
	p := suyac.Project{
		Name:      projName,
		Templates: map[string]suyac.RequestTemplate{},
		Flows:     map[string]suyac.Flow{},
		Vars:      suyac.NewVarStore(),
		History:   []suyac.HistoryEntry{},
		Session:   suyac.Session{},
		Config: suyac.Settings{
			CookieLifetime: 24 * time.Hour,
			ProjFile:       suyac.DefaultProjectPath,
			HistFile:       suyac.DefaultHistoryPath,
			SeshFile:       suyac.DefaultSessionPath,
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

	return p.PersistToDisk(true)
}
