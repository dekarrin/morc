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
	FlowCmd.PersistentFlags().StringArrayVarP(&flagFlowStepMoves, "move", "m", nil, "Move the step at index FROM to index TO. Argument must be a string in form `FROM:[TO]`. Can be given multiple times; if so, will be applied in order given after any removals and adds are applied. If TO is not given, the step is moved to the end of the flow.")

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
		disallowStepMutations := func(reason string) error {
			if len(flagFlowStepRemovals) > 0 {
				return fmt.Errorf("--remove/-r: step removal %s", reason)
			}
			if len(flagFlowStepAdds) > 0 {
				return fmt.Errorf("--add/-a: step addition %s", reason)
			}
			if len(flagFlowStepMoves) > 0 {
				return fmt.Errorf("--move/-m: step moving %s", reason)
			}

			return nil
		}

		opts := flowsOptions{
			projFile:         commonflags.ProjectFile,
			stepReplacements: map[int]string{},
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// flag sanity checks
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

			if err := disallowStepMutations("requires name of flow to modify"); err != nil {
				return err
			}
		} else if len(args) == 1 {
			flowName = args[0]

			if flagFlowDelete {
				opts.action = flowsDelete
			} else if len(flagFlowStepAdds) > 0 || len(flagFlowStepRemovals) > 0 || len(flagFlowStepMoves) > 0 {
				// we are actually in edit mode due to edit flags specified.
				opts.action = flowsEdit
			} else {
				opts.action = flowsShow
			}
		} else {
			flowName = args[0]

			if len(args) == 2 {
				if err := disallowStepMutations("cannot be performed while getting flow step/attribute"); err != nil {
					return err
				}
			}

			if flagFlowNew {
				opts.action = flowsNew

				// already checked required arg count for --new above; no need to do so again
				steps = args[1:]

				for _, reqName := range steps {
					if reqName == "" {
						return fmt.Errorf("request name cannot be empty")
					}
				}
			} else {
				// full arg parsing mode
				var curKey flowKey
				var err error

				for i, arg := range args[1:] {
					if i%2 == 0 {
						// if even, should be an attribute or a step index.

						curKey, err = parseFlowAttrKey(arg)
						if err != nil {
							return fmt.Errorf("attr/idx #%d: %w", (i/2)+1, err)
						}

						// do "already set" check
						setTwice := false

						switch curKey.name {
						case "":
							idx := curKey.stepIndex
							_, setTwice = opts.stepReplacements[idx]
						case flowKeyName.name:
							setTwice = opts.newName.set
						default:
							return fmt.Errorf("attr/idx #%d: unknown attribute %q", (i/2)+1, curKey)
						}

						if setTwice {
							return fmt.Errorf("%s is set more than once", curKey.Human())
						}
					} else {
						// if odd, it is a value

						switch curKey.name {
						case "":
							idx := curKey.stepIndex
							opts.stepReplacements[idx] = arg
						case flowKeyName.name:
							opts.newName = optional[string]{set: true, v: arg}
						default:
							panic(fmt.Sprintf("unhandled flow key %q", curKey))
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

		// okay, now bring in step modifications. we should have done enough
		// checks by now to now that these will only be set when valid so for
		// the -m, -r, and -a flag parsing below, we just panic if a sanity
		// check fails

		if len(flagFlowStepRemovals) > 0 {
			if opts.action != flowsEdit {
				panic(fmt.Sprintf("should never happen: step removals parsing reached for action %d", opts.action))
			}
			opts.stepRemovals = flagFlowStepRemovals
			sort.Sort(sort.Reverse(sort.IntSlice(opts.stepRemovals)))
		}

		addsByIndex := map[int][]flowStepAdd{}
		sortedAddIndexes := []int{}

		if len(flagFlowStepAdds) > 0 {
			if opts.action != flowsEdit {
				panic(fmt.Sprintf("should never happen: step adds parsing reached for action %d", opts.action))
			}
			for addIdx, step := range flagFlowStepAdds {
				var add flowStepAdd
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
					addsByIndex[add.index] = []flowStepAdd{}
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
				opts.stepAdds = append(opts.stepAdds, addsByIndex[idx]...)
			}

			// now add index -1's
			if addsByIndex[-1] != nil {
				opts.stepAdds = append(opts.stepAdds, addsByIndex[-1]...)
			}
		}

		if len(flagFlowStepMoves) > 0 {
			if opts.action != flowsEdit {
				panic(fmt.Sprintf("should never happen: step moves parsing reached for action %d", opts.action))
			}

			for moveIdx, move := range flagFlowStepMoves {
				parts := strings.Split(move, ":")
				if len(parts) != 2 {
					// only allowed if TO is omitted, but maybe they forgot the trailing colon. be nice and add it for
					// them and try again
					if len(parts) == 1 {
						parts = strings.Split(move+":", ":")
					} else {
						return fmt.Errorf("move #%d: invalid format %q", moveIdx+1, move)
					}
				}

				from, err := strconv.Atoi(parts[0])
				if err != nil {
					return fmt.Errorf("move #%d: FROM is not a valid step index: %s", moveIdx+1, parts[0])
				}

				var to int
				if parts[1] == "" {
					to = -1
					// move to end of slice
				} else {
					to, err = strconv.Atoi(parts[1])
					if err != nil {
						return fmt.Errorf("move #%d: TO is not a valid step index: %s", moveIdx+1, parts[1])
					}

					if to < 0 {
						to = -1
					}
				}

				opts.stepMoves = append(opts.stepMoves, flowStepMove{from: from, to: to})
			}

		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch opts.action {
		case flowsList:
			return invokeFlowsList(io, opts)
		case flowsShow:
			return invokeFlowsShow(io, flowName, opts)
		case flowsDelete:
			return invokeFlowsDelete(io, flowName, opts)
		case flowsEdit:
			return invokeFlowsEdit(io, flowName, opts)
		case flowsGet:
			return invokeFlowsGet(io, flowName, getItem, opts)
		case flowsNew:
			return invokeFlowsNew(io, flowName, steps, opts)

		default:
			panic(fmt.Sprintf("unhandled flow action %q", opts.action))
		}
	},
}

type optional[E any] struct {
	set bool
	v   E
}

func invokeFlowsEdit(io cmdio.IO, flowName string, opts flowsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	flowName = strings.ToLower(flowName)
	flow, ok := p.Flows[flowName]
	if !ok {
		return fmt.Errorf("no flow named %s exists", flowName)
	}

	modifiedVals := map[cmdio.AttrKey]interface{}{}
	noChangeVals := map[cmdio.AttrKey]interface{}{}

	if opts.newName.set {
		newNameLower := strings.ToLower(opts.newName.v)
		if newNameLower != flowName {
			if newNameLower == "" {
				return fmt.Errorf("new name cannot be empty")
			}
			if _, exists := p.Flows[newNameLower]; exists {
				return fmt.Errorf("flow named %s already exists", opts.newName.v)
			}

			flow.Name = newNameLower
			delete(p.Flows, flowName)
			modifiedVals[flowKeyName] = newNameLower
		} else {
			noChangeVals[flowKeyName] = flowName
		}
	}

	for _, delIdx := range opts.stepRemovals {
		if delIdx < 1 || delIdx > len(flow.Steps) {
			return fmt.Errorf("cannot delete step #%d; it does not exist", delIdx)
		}
		actualIdx := delIdx - 1

		newSteps := make([]morc.FlowStep, len(flow.Steps)-1)
		copy(newSteps, flow.Steps[:actualIdx])
		copy(newSteps[actualIdx:], flow.Steps[actualIdx+1:])
		flow.Steps = newSteps
	}

	for _, add := range opts.stepAdds {
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
			if actualIdx < len(newSteps) {
				copy(newSteps[actualIdx+1:], flow.Steps[actualIdx:])
			}

			flow.Steps = newSteps
		}
	}

	for _, move := range opts.stepMoves {
		// arg check; is this possible?
		if move.from < 1 || move.from > len(flow.Steps) {
			return fmt.Errorf("cannot move step #%d; it does not exist", move.from)
		}
		if move.to < -1 {
			return fmt.Errorf("cannot move step #%[1]d to #%[2]d; #%[2]d does not exist", move.from, move.to)
		}
		if move.to > len(flow.Steps) {
			move.to = -1
		}

		if move.from != move.to {
			fromPos := move.from - 1

			moved := flow.Steps[fromPos]

			// first, erase the old to move everyfin from old pos + 1 down by
			// one.
			stepsWithoutMoved := make([]morc.FlowStep, len(flow.Steps)-1)
			copy(stepsWithoutMoved, flow.Steps[:fromPos])
			copy(stepsWithoutMoved[fromPos:], flow.Steps[fromPos+1:])

			var newSteps []morc.FlowStep
			if move.to == -1 || move.to-1 == len(stepsWithoutMoved) {
				// append operation
				stepsWithoutMoved = append(stepsWithoutMoved, moved)
				newSteps = stepsWithoutMoved
			} else {
				toPos := move.to - 1
				newSteps = make([]morc.FlowStep, len(stepsWithoutMoved)+1)

				if toPos > 0 {
					copy(newSteps, stepsWithoutMoved[:toPos])
				}
				newSteps[toPos] = moved

				// if swap is at end, there's nothing to move up (toPos+1 will
				// be invalid)

				if toPos < len(newSteps) {
					copy(newSteps[toPos+1:], stepsWithoutMoved[toPos:])
				}
			}

			flow.Steps = newSteps
		} else {
			// noChange add.
		}
	}

	p.Flows[flow.Name] = flow

	return p.PersistToDisk(false)
}

func invokeFlowsGet(io cmdio.IO, flowName string, getItem cmdio.AttrKey, opts flowsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names

	flowName = strings.ToUpper(flowName)
	flow, ok := p.Flows[flowName]
	if !ok {
		return fmt.Errorf("no flow named %s exists", flowName)
	}

	switch getItem {
	case flowKeyName:
		io.Printf("%s\n", flow.Name)
	default:
		// it's a step index (panic if not)
		stepIdx, ok := getItem.(flowStepIndex)
		if !ok {
			panic("failed to cast flowStepIndex")
		}
		idx := int(stepIdx)

		if idx < 1 {
			return fmt.Errorf("%s index must be greater than 0")
		}

		if idx >= len(flow.Steps) {
			return fmt.Errorf("%d doesn't exist; highest step index in %s is %d", idx, flow.Name, len(flow.Steps)-1)
		}

		io.Printf("%s\n", flow.Steps[idx])
	}

	return nil
}

func invokeFlowsNew(io cmdio.IO, name string, templates []string, opts flowsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	name = strings.ToLower(name)

	// check if the project already has a flow with the same name
	if _, exists := p.Flows[name]; exists {
		return fmt.Errorf("flow %s already exists in project", name)
	}

	// check that each of the templates exist and create the flow steps
	var steps []morc.FlowStep
	for _, reqName := range templates {
		reqLower := strings.ToLower(reqName)
		if _, exists := p.Templates[reqLower]; !exists {
			return fmt.Errorf("no request template %q in project", reqName)
		}
		steps = append(steps, morc.FlowStep{
			Template: reqLower,
		})
	}

	// create the new flow
	flow := morc.Flow{
		Name:  name,
		Steps: steps,
	}

	p.Flows[name] = flow

	// save the project file
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Created new flow %s with %s\n", name, io.CountOf(len(templates), "step"))

	return nil
}

func invokeFlowsShow(io cmdio.IO, name string, opts flowsOptions) error {
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

	if len(flow.Steps) == 0 {
		io.Println("(no steps in flow)")
	}

	for i, step := range flow.Steps {
		req, exists := p.Templates[step.Template]

		if exists {
			notSendableBang := ""
			meth := req.Method
			reqURL := req.URL
			if meth == "" {
				notSendableBang = "!"
				meth = "???"
			}
			if reqURL == "" {
				notSendableBang = "!"
				reqURL = "http://???"
			}

			io.Printf("%d:%s %s (%s %s)\n", i+1, notSendableBang, step.Template, meth, reqURL)
		} else {
			io.Printf("%d:! %s (!non-existent req)\n", i+1, step.Template)
		}
	}

	return nil
}

func invokeFlowsList(io cmdio.IO, opts flowsOptions) error {
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

// probs overengineered given there is ONE flow attribute constant other than
// steps. If name is "" it is assumed that it is an index-style attribute.
type flowKey struct {
	name      string
	stepIndex int
}

func (fk flowKey) isStepIndex() bool {
	return fk.name == ""
}

var (
	flowKeyName flowKey = flowKey{name: "NAME"}
)

// Human prints the human-readable description of the key.
func (fk flowKey) Human() string {
	switch fk.name {
	case "":
		return fmt.Sprintf("step %d", fk.stepIndex)
	case flowKeyName.name:
		return "flow name"
	default:
		return fmt.Sprintf("unknown flow key %q", fk.name)
	}
}

func (fk flowKey) Name() string {
	if fk.name != "" {
		return string(fk.name)
	} else {
		return fmt.Sprintf("%d", fk.stepIndex)
	}
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
	if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
		idx, err := strconv.Atoi(s)
		if err != nil {
			return flowKey{}, fmt.Errorf("must be a step index or one of %s", s, strings.Join(flowAttrKeyNames(), ", "))
		}

		return flowKey{stepIndex: idx}, nil
	}

	switch strings.ToUpper(s) {
	case flowKeyName.Name():
		return flowKeyName, nil
	default:
		return flowKey{}, fmt.Errorf("must be a step index or one of %s", s, strings.Join(flowAttrKeyNames(), ", "))
	}
}

type flowStepAdd struct {
	index    int
	template string
}

type flowStepMove struct {
	from int
	to   int
}

type flowsOptions struct {
	projFile string
	action   flowAction

	newName          optional[string]
	stepReplacements map[int]string
	stepAdds         []flowStepAdd
	stepRemovals     []int
	stepMoves        []flowStepMove
}
