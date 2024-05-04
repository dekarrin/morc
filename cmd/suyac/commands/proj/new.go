package proj

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagHistoryFile    string
	flagSessionFile    string
	flagCookieLifetime string
)

func init() {
	newCmd.LocalFlags().StringVarP(&flagHistoryFile, "history", "H", "", "Create and save history in the given file")
	newCmd.LocalFlags().StringVarP(&flagSessionFile, "session", "S", "", "Create and save session in the given file")
	newCmd.LocalFlags().StringVarP(&flagCookieLifetime, "cookie-lifetime", "C", "24h", "Set the lifetime of recorded Set-Cookie calls in notation like \"24h\" or \"1h30m\"")

	RootCmd.AddCommand(newCmd)
}

var newCmd = &cobra.Command{
	Use:   "new NAME [-F file] [-H file] [-S file] [-C duration]",
	Short: "Create a new suyac project file",
	Long:  "Create a new suyac project file. Only the project file is created, not a session or history file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]

		opts := newOptions{
			filename: flagProjectFile,
			histFile: flagHistoryFile,
			seshFile: flagSessionFile,
		}

		var err error
		opts.cookieLifetime, err = time.ParseDuration(flagCookieLifetime)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}

		if opts.filename == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// make absolute paths
		opts.filename, err = filepath.Abs(opts.filename)
		if err != nil {
			return fmt.Errorf("could not get absolute path for project file: %w", err)
		}

		return invokeNew(projectName, opts)
	},
}

type newOptions struct {
	filename       string
	histFile       string
	seshFile       string
	cookieLifetime time.Duration
}

func invokeNew(name string, opts newOptions) error {
	p := suyac.Project{
		Name:      name,
		Templates: map[string]suyac.RequestTemplate{},
		Flows:     map[string]suyac.Flow{},
		Vars:      suyac.NewVarStore(),
		History:   []suyac.HistoryEntry{},
		Session:   suyac.Session{},
		Config: suyac.Settings{
			CookieLifetime: opts.cookieLifetime,
			ProjFile:       opts.filename,
			HistFile:       opts.histFile,
			SeshFile:       opts.seshFile,
		},
	}

	err := p.PersistToDisk(false)
	if err != nil {
		return err
	}

	if opts.histFile != "" {
		err = p.PersistHistoryToDisk()
		if err != nil {
			return err
		}
	}

	if opts.seshFile != "" {
		err = p.PersistSessionToDisk()
		if err != nil {
			return err
		}
	}

	return nil
}
