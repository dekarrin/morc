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

	varS := "s"
	if proj.Vars.Count() == 1 {
		varS = ""
	}

	envS := "s"
	if proj.Vars.EnvCount() == 1 {
		envS = ""
	}

	requestS := "s"
	if len(proj.Templates) == 1 {
		requestS = ""
	}

	flowS := "s"
	if len(proj.Flows) == 1 {
		flowS = ""
	}

	cookieS := "s"
	if proj.Session.TotalCookieSets() == 1 {
		cookieS = ""
	}

	histS := "s"
	if len(proj.History) == 1 {
		histS = ""
	}

	io.Printf("Project: %s\n", proj.Name)
	io.Printf("%d request%s, %d flow%s\n", len(proj.Templates), requestS, len(proj.Flows), flowS)
	io.Printf("%d history item%s\n", len(proj.History), histS)
	io.Printf("%d variable%s across %d environment%s\n", proj.Vars.Count(), varS, proj.Vars.EnvCount(), envS)
	io.Printf("%d cookie%s in active session\n", proj.Session.TotalCookieSets(), cookieS)
	io.Println()
	io.Printf("Cookie record lifetime: %s\n", proj.Config.CookieLifetime)
	io.Printf("Project file on record: %s\n", proj.Config.ProjFile)
	io.Printf("Session file on record: %s\n", proj.Config.SeshFile)
	io.Printf("History file on record: %s\n", proj.Config.HistFile)
	io.Println()
	if proj.Vars.Environment == "" {
		io.Printf("Using default var environment\n")
	} else {
		io.Printf("Using var environment %s\n", proj.Vars.Environment)
	}

	return nil
}
