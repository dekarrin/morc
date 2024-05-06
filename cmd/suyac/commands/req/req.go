package req

import (
	"fmt"
	"sort"

	"github.com/dekarrin/suyac"
	"github.com/dekarrin/suyac/cmd/suyac/commands/req/caps"
	"github.com/dekarrin/suyac/cmd/suyac/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&commonflags.ReqProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)

	RootCmd.AddCommand(caps.RootCmd)
}

var RootCmd = &cobra.Command{
	Use:     "req [-F project_file]",
	GroupID: "project",
	Short:   "Show or manipulate request templates",
	Long:    "Print out a listing of the names and methods of the request templates in the project.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := commonflags.ReqProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}
		return listRequests(filename)
	},
}

func listRequests(filename string) error {
	p, err := suyac.LoadProjectFromDisk(filename, true)
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
