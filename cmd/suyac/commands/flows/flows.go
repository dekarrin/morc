package flows

import (
	"fmt"
	"sort"

	"github.com/dekarrin/suyac"
	"github.com/dekarrin/suyac/cmd/suyac/commands/reqs/caps"
	"github.com/dekarrin/suyac/cmd/suyac/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&commonflags.ReqProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)

	RootCmd.AddCommand(caps.RootCmd)
}

var RootCmd = &cobra.Command{
	Use:     "flows [-F project_file]",
	GroupID: "project",
	Short:   "Show or manipulate request flows",
	Long:    "By itself, prints out a listing of all flows that are in the project. With subcommands, manipulates them.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := commonflags.ReqProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeFlowsList(filename)
	},
}

func invokeFlowsList(filename string) error {
	p, err := suyac.LoadProjectFromDisk(filename, false)
	if err != nil {
		return err
	}

	if len(p.Flows) == 0 {
		fmt.Println("(none)")
	} else {
		// alphabetize the flows
		var sortedNames []string
		for name := range p.Flows {
			sortedNames = append(sortedNames, name)
		}
		sort.Strings(sortedNames)

		for _, name := range sortedNames {
			f := p.Flows[name]

			reqS := "s"
			if len(f.Steps) == 1 {
				reqS = ""
			}

			fmt.Printf("%s: %d request%s\n", f.Name, len(f.Steps), reqS)
		}
	}

	return nil
}