package proj

import (
	"fmt"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

type projAction int

const (
	projInfo projAction = iota
	projNew
	projEdit
)

var (
	flagProjHistoryFile    string
	flagProjSessionFile    string
	flagProjCookieLifetime string
	flagProjRecordCookies  bool
	flagProjRecordHistory  bool
)

func init() {
	// TODO: get specific item flags

	RootCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)

	RootCmd.PersistentFlags().StringVarP(&flagProjHistoryFile, "history-file", "H", "", "Show the currently-set path for the history file.\nWhen used with --edit or --new: Set history file to `PATH`. Does not affect whether history is actually recorded; use --hist-on and --hist-off for that. If the special string '"+morc.ProjDirVar+"' is in the path given, it is replaced with the relative directory of the project file whenever morc is executed.")
	RootCmd.PersistentFlags().StringVarP(&flagProjSessionFile, "session-file", "S", "", "Show the currently-set path for the session file.\nWhen used with --edit or --new: Set session file to `PATH`. Does not affect whether session data (cookies) is actually recorded; use --cookies-on and --cookies-off for that.\nIf the special string '"+morc.ProjDirVar+"' is in the path given, it is replaced with the relative directory of the project file whenever morc is executed.")
	RootCmd.PersistentFlags().StringVarP(&flagProjCookieLifetime, "cookie-lifetime", "C", "24h", "Get the lifetime of recorded Set-Cookie calls.\nWhen used with --edit or --new: Set the lifetime of recorded Set-Cookie calls to the given duration. `DUR` must be a string in notation like \"24h\" or \"1h30m\". If set to 0 or less, will be interpreted as 24h. Altering this will immediately apply an eviction check to all current cookies; this may result in some being purged.")
	RootCmd.PersistentFlags().BoolVar(&flagProjRecordCookies, "cookies", false, "Show whether cookie recording is currently enabled.\nWhen used with --edit or --new: Enable or disable cookie recording. `OFF/ON` must be either the string \"OFF\" or \"ON\", case-insensitive. Equivalent to 'morc cookies --on' or 'morc cookies --off'.")
	RootCmd.PersistentFlags().BoolVar(&flagProjRecordHistory, "history", false, "Show whether history recording is currently enabled.\nWhen used with --edit or --new: Enable or disable history recording. `OFF/ON` must be either the string \"OFF\" or \"ON\", case-insensitive. Equivalent to 'morc hist --on' or 'morc hist --off'.")

	RootCmd.PersistentFlags().Lookup("history-file").NoOptDefVal = morc.ProjDirVar
}

var RootCmd = &cobra.Command{
	Use:     "proj",
	GroupID: "project",
	Short:   "Show or manipulate project attributes and config",
	Long:    "Show the contents of the morc project referred to in the .morc dir in the current directory, or use -F to read a file in another location.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := commonflags.ProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		return invokeProjShow(io, filename)
	},
}

func invokeProjShow(io cmdio.IO, filename string) error {
	proj, err := morc.LoadProjectFromDisk(filename, true)
	if err != nil {
		return err
	}

	io.Printf("Project: %s\n", proj.Name)
	io.Printf("%s, %s\n", io.CountOf(len(proj.Templates), "request"), io.CountOf(len(proj.Flows), "flow"))
	io.Printf("%s\n", io.CountOf(len(proj.History), "history item"))
	io.Printf("%s across %s\n", io.CountOf(proj.Vars.Count(), "variable"), io.CountOf(proj.Vars.EnvCount(), "environment"))
	io.Printf("%s in active session\n", io.CountOf(proj.Session.TotalCookieSets(), "cookie"))
	io.Println()
	io.Printf("Cookie record lifetime: %s\n", proj.Config.CookieLifetime)
	io.Printf("Project file on record: %s\n", proj.Config.ProjFile)
	io.Printf("Session file on record: %s\n", proj.Config.SeshFile)
	io.Printf("History file on record: %s\n", proj.Config.HistFile)
	io.Printf("Cookie recording is %s\n", io.OnOrOff(proj.Config.RecordSession))
	io.Printf("History tracking is %s\n", io.OnOrOff(proj.Config.RecordHistory))
	io.Println()
	if proj.Vars.Environment == "" {
		io.Printf("Using default var environment\n")
	} else {
		io.Printf("Using var environment %s\n", proj.Vars.Environment)
	}

	return nil
}
