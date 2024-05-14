package proj

import (
	"fmt"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagProjectFile string
)

func init() {
	// TODO: specific item flags

	RootCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
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
