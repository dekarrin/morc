package proj

import (
	"fmt"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagProjectFile string
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&flagProjectFile, "file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
}

var RootCmd = &cobra.Command{
	Use:     "proj",
	GroupID: "project",
	Short:   "Show or manipulate project attributes and config",
	Long:    "Show the contents of the suyac project referred to in the .suyac dir in the current directory, or use -F to read a file in another location.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := flagProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}
		return invokeShow(filename)
	},
}

func invokeShow(filename string) error {
	proj, err := suyac.LoadProjectFromDisk(filename, true)
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

	fmt.Printf("Project: %s\n", proj.Name)
	fmt.Printf("%d request%s, %d flow%s\n", len(proj.Templates), requestS, len(proj.Flows), flowS)
	fmt.Printf("%d history item%s\n", len(proj.History), histS)
	fmt.Printf("%d variable%s across %d environment%s\n", proj.Vars.Count(), varS, proj.Vars.EnvCount(), envS)
	fmt.Printf("%d cookie%s in active session\n", proj.Session.TotalCookieSets(), cookieS)
	fmt.Println()
	fmt.Printf("Cookie record lifetime: %s\n", proj.Config.CookieLifetime)
	fmt.Printf("Project file on record: %s\n", proj.Config.ProjFile)
	fmt.Printf("Session file on record: %s\n", proj.Config.SeshFile)
	fmt.Printf("History file on record: %s\n", proj.Config.HistFile)
	fmt.Println()
	if proj.Vars.Environment == "" {
		fmt.Printf("Using default var environment\n")
	} else {
		fmt.Printf("Using var environment %s\n", proj.Vars.Environment)
	}

	return nil
}
