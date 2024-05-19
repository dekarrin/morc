package flows

import (
	"fmt"
	"sort"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
}

var RootCmd = &cobra.Command{
	Use: "flows [-F FILE]\n" +
		"flows FLOW --new REQ1 REQ2 [REQN]... [-F FILE]\n" +
		"flows FLOW [-F FILE]\n" +
		"flows FLOW -d [-F FILE]\n" +
		"flows FLOW ATTR/IDX [-F FILE]\n" +
		"flows FLOW [ATTR/IDX VAL]... [-r IDX]... [-a [IDX]:REQ]... [-m IDX->IDX]... [-F FILE]",
	GroupID: "project",
	Short:   "Get or modify request flows",
	Long:    "Performs operations on the flows defined in the project. By itself, lists out the names of all flows in the project. If given a flow name FLOW with no other arguments, shows the steps in the flow. A new flow can be created by including the --new flag when providing the name of the flow and 2 or more names of requests to be included, in order. A flow can be deleted by passing the -d flag when providing the name of the flow. If a numerical flow step index IDX is provided after the flow name, the name of the req at that step is output. If a non-numerical flow attribute ATTR is provided after the flow name, that attribute is output. If a value is provided after ATTR or IDX, the attribute or step at the given index is updated to the new value. Format for the new value for an ATTR is dependent on the ATTR, and format for the new value for an IDX is the name of the request to call at that step index.\n\nFlow step mutations other than a step replacing an existing one are handled by giving the name of the FLOW and one or more step mutation options. --remove/-r IDX can be used to remove the step at the given index. --add/-a [IDX]:REQ will add a new step at the given index, or at the end if IDX is omitted; double the colon to insert a template whose name begins with a colon at the end of the flow.. --move/-m IDX->IDX will move the step at the first index to the second index; if the new index is higher than the old, all indexes in between move down to accommodate, and if the new index is lower, all other indexes are pushed up to accommodate. Multiple moves, adds, and removes can be given in a single command; all removes are applied from highest to lowest index, then any adds from lowest to highest, then any moves.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := commonflags.ProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// semantic CLI actions (a little weird because flow contains a list of steps):
		// flows - LIST
		// flows FLOW - SHOW
		// flows FLOW STEP1REQ STEP2REQ [STEPNREQ]... --new  - NEW
		// flows FLOW -d - DELETE

		// (GET?) NAME, STEPS, N.
		// (EDIT?) NAME=NEW/+N NEW/+-r N/+-a N:NEW/+-m N->M

		// mutation steps are applicable in EDIT-style, SHOW-style, INCOMPAT with -d,

		// * if user gives new, must have at least 3 args and no -d and no step mutations
		// * if user gives -d, must have 1 arg and no step mutations and no --new.
		//
		// * if user puts no args, action is LIST. no args beside -F are allowed.
		// * if user puts 1 arg, it is flow name. -d and -F is allowed. step mutations are allowed, but not with -d.
		//    * if -d is present, action is DELETE
		//    * if step mutations are present, action is EDIT
		//    * else, action is SHOW
		// * if user puts 2 args, first is flow name.
		//    * if 2nd arg is numeric, it is an index.
		//    * else, it is an attribute name.
		//    * action is GET ITEM (roll parsing into below)
		// * if user puts 2 or more args, first is flow name.
		//    * if --new is set, action is NEW. 2nd, 3rd, and following args are request names
		//    * else, parse args as edit attrs.
		//    * if arg count is 1, action is GET ITEM. Otherwise, action is EDIT.
		//    * if final arg count is odd and > 1, ERROR

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
