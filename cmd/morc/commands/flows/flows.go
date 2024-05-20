package flows

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagFlowNew          bool
	flagFlowDelete       bool
	flagFlowStepRemovals []int
	flagFlowStepAdds     []string
	flagFlowStepMoves    []string
)

func init() {
	FlowCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	FlowCmd.PersistentFlags().BoolVarP(&flagFlowDelete, "delete", "d", false, "Delete the flow with the given name. Can only be used when flow name is also given.")
	FlowCmd.PersistentFlags().BoolVarP(&flagFlowNew, "new", "", false, "Create a new flow with the given name and request steps. If given, arguments to the command are interpreted as the new flow name and the request steps, in order.")
	FlowCmd.PersistentFlags().IntSliceVarP(&flagFlowStepRemovals, "remove", "r", nil, "Remove the step at index `IDX` from the flow. Can be given multiple times; if so, will be applied from highest to lowest index.")
	FlowCmd.PersistentFlags().StringArrayVarP(&flagFlowStepAdds, "add", "a", nil, "Add a new step calling request REQ at index IDX, or at the end of current steps if index is omitted. Argument must be a string in form `[IDX]:REQ`. Can be given multiple times; if so, will be applied from lowest to highest index after any removals are applied.")
	FlowCmd.PersistentFlags().StringArrayVarP(&flagFlowStepMoves, "move", "m", nil, "Move the step at index FROM to index TO. Argument must be a string in form `FROM:TO`. Can be given multiple times; if so, will be applied in order given after any removals and adds are applied.")

	FlowCmd.MarkFlagsMutuallyExclusive("delete", "new", "remove")
	FlowCmd.MarkFlagsMutuallyExclusive("delete", "new", "add")
	FlowCmd.MarkFlagsMutuallyExclusive("delete", "new", "move")
}

var FlowCmd = &cobra.Command{
	Use: "flows [-F FILE]\n" +
		"flows FLOW --new REQ1 REQ2 [REQN]... [-F FILE]\n" +
		"flows FLOW [-F FILE]\n" +
		"flows FLOW -d [-F FILE]\n" +
		"flows FLOW ATTR/IDX [-F FILE]\n" +
		"flows FLOW [ATTR/IDX VAL]... [-r IDX]... [-a [IDX]:REQ]... [-m FROM:TO]... [-F FILE]",
	GroupID: "project",
	Short:   "Get or modify request flows",
	Long:    "Performs operations on the flows defined in the project. By itself, lists out the names of all flows in the project. If given a flow name FLOW with no other arguments, shows the steps in the flow. A new flow can be created by including the --new flag when providing the name of the flow and 2 or more names of requests to be included, in order. A flow can be deleted by passing the -d flag when providing the name of the flow. If a numerical flow step index IDX is provided after the flow name, the name of the req at that step is output. If a non-numerical flow attribute ATTR is provided after the flow name, that attribute is output. If a value is provided after ATTR or IDX, the attribute or step at the given index is updated to the new value. Format for the new value for an ATTR is dependent on the ATTR, and format for the new value for an IDX is the name of the request to call at that step index.\n\nFlow step mutations other than a step replacing an existing one are handled by giving the name of the FLOW and one or more step mutation options. --remove/-r IDX can be used to remove the step at the given index. --add/-a [IDX]:REQ will add a new step at the given index, or at the end if IDX is omitted; double the colon to insert a template whose name begins with a colon at the end of the flow.. --move/-m IDX->IDX will move the step at the first index to the second index; if the new index is higher than the old, all indexes in between move down to accommodate, and if the new index is lower, all other indexes are pushed up to accommodate. Multiple moves, adds, and removes can be given in a single command; all removes are applied from highest to lowest index, then any adds from lowest to highest, then any moves.",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := flowOptions{
			projFile:         commonflags.ProjectFile,
			stepReplacements: map[int]string{},
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// semantic CLI actions (a little weird because flow contains a list of steps):
		// flows - LIST
		// flows FLOW - SHOW
		// flows FLOW STEP1REQ STEP2REQ [STEPNREQ]... --new  - NEW
		// flows FLOW -d - DELETE

		// (GET?) NAME, STEPS, N.
		// (EDIT?) NAME=NEW/+N NEW/+-r N/+-a N:NEW/+-m N:M

		// mutation steps are applicable in EDIT-style, SHOW-style, INCOMPAT with -d,

		// sanity flag checks
		if flagFlowNew {
			if len(args) < 3 {
				return fmt.Errorf("--new requires a name and at least two requests")
			}
		}
		if flagFlowDelete {
			if len(args) < 1 {
				return fmt.Errorf("--delete/-d requires a flow name")
			}
			if len(args) > 1 {
				return fmt.Errorf("--delete/-d must be used only with a flow name")
			}
		}

		var flowName string
		var steps []string
		var getItem cmdio.AttrKey

		if len(args) == 0 {
			opts.action = flowsList

			if len(flagFlowStepRemovals) > 0 {
				return fmt.Errorf("--remove/-r: step removal requires name of flow to remove from")
			}
			if len(flagFlowStepAdds) > 0 {
				return fmt.Errorf("--add/-a: step addition requires name of flow to add to")
			}
			if len(flagFlowStepMoves) > 0 {
				return fmt.Errorf("--move/-m: step moving requires name of flow to move within")
			}
		} else if len(args) == 1 {
			flowName = args[0]

			if flagFlowDelete {
				opts.action = flowsDelete
			} else {
				opts.action = flowsShow
			}
		} else {
			flowName = args[0]

			if len(args) == 2 {
				if len(flagFlowStepRemovals) > 0 {
					return fmt.Errorf("--remove/-r: step removal cannot be performed while getting flow step/attribute")
				}
				if len(flagFlowStepAdds) > 0 {
					return fmt.Errorf("--add/-a: step addition cannot be performed while getting flow step/attribute")
				}
				if len(flagFlowStepMoves) > 0 {
					return fmt.Errorf("--move/-m: step moving cannot be performed while getting flow step/attribute")
				}
			}

			if flagFlowNew {
				opts.action = flowsNew

				// already checked required arg count for --new above; no need to do so again
				steps = args[1:]
			} else {
				// full arg parsing mode
				var curKey cmdio.AttrKey
				var curKeyIsIndex bool
				var err error

				for i, arg := range args[1:] {
					if i%2 == 0 {
						// if even, should be an attribute or a step index.

						curKeyIsIndex = false

						// as a rule, no non-index attribute begins with a
						// digit, so attempt to parse as an index first.
						if len(arg) > 0 && arg[0] >= '0' && arg[0] <= '9' {
							idx, err := strconv.Atoi(arg)
							if err != nil {
								return fmt.Errorf("attr/idx #%d: not a valid step index: %w", (i/2)+1, err)
							}

							curKey = flowStepIndex(idx)
							curKeyIsIndex = true
						}

						// if index is not set, it must be an attribute. parse.
						if !curKeyIsIndex {
							curKey, err = parseFlowAttrKey(arg)
							if err != nil {
								return fmt.Errorf("attr/idx #%d: %w", (i/2)+1, err)
							}
						}

						// do "already set" check
						setTwice := false
						switch curKey {
						case flowKeyName:
							setTwice = opts.newName.set
						default:
							// check in the step replacements
							idx, ok := curKey.(flowStepIndex)
							if !ok {
								return fmt.Errorf("attr/idx #%d: unknown attribute %q", (i/2)+1, curKey)
							}
							_, setTwice = opts.stepReplacements[int(idx)]
						}

						if setTwice {
							return fmt.Errorf("%s is set more than once", curKey.Human())
						}
					} else {
						// if odd, it is a value

						switch curKey {
						case flowKeyName:
							opts.newName = optional[string]{set: true, v: arg}
						default:
							idx, ok := curKey.(flowStepIndex)
							if !ok {
								panic("failed to cast flowStepIndex")
							}
							opts.stepReplacements[int(idx)] = arg
						}
					}
				}

				// now that we are done, do an arg-count check and use it to set
				// action.
				// doing AFTER parsing so that we can give a betta error message if
				// missing last value
				if len(args[1:]) == 1 {
					// that's fine, we just want to get the one item
					opts.action = flowsGet
					getItem = curKey
				} else if len(args)%2 != 0 {
					return fmt.Errorf("%s is missing a value", curKey)
				} else {
					opts.action = flowsEdit
				}
			}
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch opts.action {
		case flowsList:
			return invokeFlowsList(io, opts)
		default:
			panic(fmt.Sprintf("unhandled flow action %q", opts.action))
		}
	},
}

func invokeFlowsList(io cmdio.IO, opts flowOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
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

type flowAction int

const (
	flowsList flowAction = iota
	flowsShow
	flowsNew
	flowsDelete
	flowsGet
	flowsEdit
)

// probs overengineered given there is ONE flow attribute other than steps.
type flowKey string

const (
	flowKeyName flowKey = "NAME"
)

// Human prints the human-readable description of the key.
func (fk flowKey) Human() string {
	switch fk {
	case flowKeyName:
		return "flow name"
	default:
		return fmt.Sprintf("unknown flow key %q", fk)
	}
}

func (fk flowKey) Name() string {
	return string(fk)
}

var (
	// ordering of flowAttrKeys in output is set here

	flowAttrKeys = []flowKey{
		flowKeyName,
	}
)

// custom type for holding referred-to index as an attribute of a flow
type flowStepIndex int

func (si flowStepIndex) Name() string {
	return fmt.Sprintf("%d", si)
}

func (si flowStepIndex) Human() string {
	return fmt.Sprintf("step %d", si)
}

func flowAttrKeyNames() []string {
	names := make([]string, len(flowAttrKeys))
	for i, k := range flowAttrKeys {
		names[i] = k.Name()
	}
	return names
}

func parseFlowAttrKey(s string) (flowKey, error) {
	switch strings.ToUpper(s) {
	case flowKeyName.Name():
		return flowKeyName, nil
	default:
		return "", fmt.Errorf("invalid attribute %q; must be one of %s", s, strings.Join(flowAttrKeyNames(), ", "))
	}
}

type flowOptions struct {
	projFile string
	action   flowAction

	newName          optional[string]
	stepReplacements map[int]string
}
