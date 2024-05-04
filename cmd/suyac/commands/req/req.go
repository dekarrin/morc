package req

import (
	"fmt"
	"sort"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagProjectFile string
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&flagProjectFile, "project_file", "P", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
}

var RootCmd = &cobra.Command{
	Use:   "req [ -P project_file ]",
	Short: "List the request templates in the project",
	Long:  "Print out a listing of the names and methods of the request templates in the project.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := flagProjectFile

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
