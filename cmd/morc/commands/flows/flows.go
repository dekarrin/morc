package flows

import (
	"fmt"
	"sort"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commands/reqs/caps"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)

	RootCmd.AddCommand(caps.RootCmd)
}

var RootCmd = &cobra.Command{
	Use:     "flows [-F project_file]",
	GroupID: "project",
	Short:   "Show or manipulate request flows",
	Long:    "By itself, prints out a listing of all flows that are in the project. With subcommands, manipulates them.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := commonflags.ProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		return invokeFlowsList(io, filename)
	},
}

func invokeFlowsList(io cmdio.IO, filename string) error {
	p, err := morc.LoadProjectFromDisk(filename, false)
	if err != nil {
		return err
	}

	if len(p.Flows) == 0 {
		io.Println("(none)")
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

			notExecableBang := ""
			if !p.IsExecableFlow(name) {
				notExecableBang = "!"
			}

			io.Printf("%s:%s %d request%s\n", f.Name, notExecableBang, len(f.Steps), reqS)
		}
	}

	return nil
}
