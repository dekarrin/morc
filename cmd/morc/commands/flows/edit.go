package flows

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagEditName        string
	flagEditDeleteSteps []int
	flagEditAddSteps    []string
)

func init() {
	editCmd.PersistentFlags().StringVarP(&flagEditName, "name", "n", "", "Change the name of the flow")
	editCmd.PersistentFlags().IntSliceVarP(&flagEditDeleteSteps, "delete-step", "", nil, "Delete the step at the given index. Can be repeated.")
	editCmd.PersistentFlags().StringSliceVarP(&flagEditAddSteps, "add-step", "", nil, "Add a step to the flow, in format [INDEX]:TEMPLATE. Can be repeated.")

	// TODO: enforce at least one flag needing to be on

	RootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit FLOW [-F project_file] [--name NAME] [--delete-step INDEX] [--add-step [INDEX]:TEMPLATE]",
	Short: "Edit steps or other properties of a flow",
	Long:  "Edit properties of a flow. The name can be changed by passing a name with --name. Steps can be deleted by passing one or more --delete-step flags with the numbers of steps to delete. New steps can be added by passing them to --add-step, in the format [INDEX]:TEMPLATE. If INDEX is omitted, the step is added to the end of the flow. If it is given, the step is inserted at that postion. The first leading colon is considered to indicate that no index is there; double the colon to insert a template whose name begins with a colon at the end of the flow.\n\nMultiple options may be specified in a single command. Deletes are applied first, from highest index to lowest, then adds, from lowest index to highest.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		flowName := args[0]
		if flowName == "" {
			return fmt.Errorf("flow name cannot be empty")
		}

		opts := editOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		if flagEditName == "" && len(flagEditDeleteSteps) == 0 && len(flagEditAddSteps) == 0 {
			return fmt.Errorf("must give one or more of --name, --delete-step, or --add-step")
		}

		if flagEditName != "" {
			opts.newName = flagEditName
		}

		if len(flagEditDeleteSteps) > 0 {
			opts.deletes = flagEditDeleteSteps
			sort.Sort(sort.Reverse(sort.IntSlice(opts.deletes)))
		}

		addsByIndex := map[int][]editAdd{}
		sortedAddIndexes := []int{}

		if len(flagEditAddSteps) > 0 {
			for addIdx, step := range flagEditAddSteps {
				var add editAdd
				parts := strings.SplitN(step, ":", 2)
				if len(parts) == 1 || len(parts) == 2 && parts[0] == "" {
					add.index = -1
				} else {
					var err error
					add.index, err = strconv.Atoi(parts[0])
					if err != nil {
						return fmt.Errorf("add #%d: invalid index %s", addIdx+1, parts[0])
					}
				}

				add.template = strings.ToLower(parts[len(parts)-1])

				if addsByIndex[add.index] == nil {
					addsByIndex[add.index] = []editAdd{}
					sortedAddIndexes = append(sortedAddIndexes, add.index)
				}

				addsByIndex[add.index] = append(addsByIndex[add.index], add)
			}

			sort.Ints(sortedAddIndexes)

			for _, idx := range sortedAddIndexes {
				// save for last, it is to the end
				if idx == -1 {
					continue
				}
				opts.adds = append(opts.adds, addsByIndex[idx]...)
			}

			// now add index -1's
			if addsByIndex[-1] != nil {
				opts.adds = append(opts.adds, addsByIndex[-1]...)
			}
		}
		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeFlowEdit(flowName, opts)
	},
}

type editAdd struct {
	index    int
	template string
}

type editOptions struct {
	projFile string
	newName  string
	deletes  []int
	adds     []editAdd
}

func invokeFlowEdit(name string, opts editOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	name = strings.ToLower(name)

	flow, ok := p.Flows[name]
	if !ok {
		return fmt.Errorf("no flow named %s exists", name)
	}

	newNameLower := strings.ToLower(opts.newName)
	if newNameLower != name {
		if newNameLower == "" {
			return fmt.Errorf("new name cannot be empty")
		}
		if _, exists := p.Flows[newNameLower]; exists {
			return fmt.Errorf("flow named %s already exists", opts.newName)
		}

		flow.Name = newNameLower
		delete(p.Flows, name)
	}

	for _, delIdx := range opts.deletes {
		if delIdx < 1 || delIdx > len(flow.Steps) {
			return fmt.Errorf("cannot delete step #%d; it does not exist", delIdx)
		}
		actualIdx := delIdx - 1

		newSteps := make([]morc.FlowStep, len(flow.Steps)-1)
		copy(newSteps, flow.Steps[:actualIdx])
		copy(newSteps[actualIdx:], flow.Steps[actualIdx+1:])
		flow.Steps = newSteps
	}

	for _, add := range opts.adds {
		if add.index < -1 {
			return fmt.Errorf("cannot add step at #%d; it does not exist", add.index)
		}

		if add.index > len(flow.Steps) {
			add.index = -1
		}

		// make shore the template exists
		add.template = strings.ToLower(add.template)
		if _, exists := p.Templates[add.template]; !exists {
			return fmt.Errorf("no request template %q in project", add.template)
		}

		if add.index == -1 {
			flow.Steps = append(flow.Steps, morc.FlowStep{
				Template: add.template,
			})
		} else {
			actualIdx := add.index - 1

			newSteps := make([]morc.FlowStep, len(flow.Steps)+1)

			if actualIdx > 0 {
				copy(newSteps, flow.Steps[:actualIdx])
			}
			newSteps[actualIdx] = morc.FlowStep{
				Template: add.template,
			}
			copy(newSteps[actualIdx+1:], flow.Steps[actualIdx:])

			flow.Steps = newSteps
		}
	}

	p.Flows[flow.Name] = flow

	return p.PersistToDisk(false)
}
