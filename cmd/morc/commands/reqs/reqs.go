package reqs

import (
	"fmt"
	"sort"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commands/reqs/caps"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)

	RootCmd.AddCommand(caps.RootCmd)
}

var RootCmd = &cobra.Command{
	Use:     "reqs [-F project_file]",
	GroupID: "project",
	Short:   "Show or manipulate request templates",
	Long:    "Print out a listing of the names and methods of the request templates in the project.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := commonflags.ProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeReqList(filename)
	},
}

func invokeReqList(filename string) error {
	p, err := morc.LoadProjectFromDisk(filename, true)
	if err != nil {
		return err
	}

	if len(p.Templates) == 0 {
		fmt.Println("(none)")
	} else {
		// alphabetize the templates
		var sortedNames []string
		for name := range p.Templates {
			sortedNames = append(sortedNames, name)
		}
		sort.Strings(sortedNames)

		for _, name := range sortedNames {
			meth := p.Templates[name].Method
			if meth == "" {
				meth = "???"
			}
			fmt.Printf("%s %s\n", name, meth)
		}
	}

	return nil
}
