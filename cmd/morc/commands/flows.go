package commands

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/dekarrin/morc/internal/sliceops"
	"github.com/spf13/cobra"
)

var (
	flagFlowNew          string
	flagFlowDelete       string
	flagFlowGet          string
	flagFlowName         string
	flagFlowStepRemovals []int
	flagFlowStepAdds     []string
	flagFlowStepMoves    []string
	flagFlowStepReplaces []string
)

var flowsCmd = &cobra.Command{
	Use: "flows [-F FILE]\n" +
		"flows [-F FILE] --delete FLOW\n" +
		"flows [-F FILE] --new FLOW REQ1 REQ2 [REQN]...\n" +
		"flows [-F FILE] FLOW\n" +
		"flows [-F FILE] FLOW --get ATTR\n" +
		"flows [-F FILE] FLOW [-nuram]...",
	GroupID: "project",
	Short:   "Get or modify request flows",
	Long: "Performs operations on the flows defined in the project. With no other arguments, a listing of all flows is shown.\n\n" +
		"A new flow can be created by providing the name of the new flow with the --new flag and providing the names of least " +
		"two requests to be included in the flow.\n\n" +
		"A flow can be examined by providing FLOW, the name of it. This will display the list of all steps in the flow. To see a particular " +
		"attribute of a flow, --get can be used to select it. --get takes either the string \"name\" to explicitly get the flow's name as " +
		"it is recorded by MORC, or the 1-based index number of a flow's step.\n\n" +
		"To modify a flow, provide the name of the FLOW and give one or more modification flags. --name/-n is used to change the name, and " +
		"can only be specified once. Steps are modified with other flags: --update/-u to change the request a step calls, --remove/-r to " +
		"remove a step, --add/-a to add a step, and --move/-m to move a step to a new position. All step-modification flags " +
		"can be specified more than once to apply multiple updates in the same call to MORC. For handling multiple types of step " +
		"modifications given in the same invocation, MORC will apply the modifications in the following order: step template updates are " +
		"applied in the order they were given in CLI flags, then all deletes are applied from highest to lowest index, followed by all adds " +
		"from lowest to to highest index, and finally all moves in the order they were given in CLI flags.\n\n" +
		"A flow is deleted by providing the --delete/-D flag with the FLOW to be deleted as its argument.",
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args flowsArgs
		if err := parseFlowsArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch args.action {
		case flowsList:
			return invokeFlowsList(io, args.projFile)
		case flowsShow:
			return invokeFlowsShow(io, args.projFile, args.flow)
		case flowsDelete:
			return invokeFlowsDelete(io, args.projFile, args.flow)
		case flowsEdit:
			return invokeFlowsEdit(io, args.projFile, args.flow, args.sets)
		case flowsGet:
			return invokeFlowsGet(io, args.projFile, args.flow, args.getItem)
		case flowsNew:
			return invokeFlowsNew(io, args.projFile, args.flow, args.reqs)

		default:
			panic(fmt.Sprintf("unhandled flow action %q", args.action))
		}
	},
}

func init() {
	flowsCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	flowsCmd.PersistentFlags().StringVarP(&flagFlowDelete, "delete", "D", "", "Delete the flow with the name `FLOW`.")
	flowsCmd.PersistentFlags().StringVarP(&flagFlowNew, "new", "N", "", "Create a new flow with the name `FLOW`. When given, positional arguments are interpreted as ordered names of requests that make up the new flow's steps. At least two requests must be present.")
	flowsCmd.PersistentFlags().StringVarP(&flagFlowGet, "get", "G", "", "Get the value of an attribute of the flow. `ATTR` can either be 'name', to get the flow name, or the 1-based index of a specific step in the flow.")
	flowsCmd.PersistentFlags().IntSliceVarP(&flagFlowStepRemovals, "remove", "r", nil, "Remove the step at index `IDX` from the flow. Can be given multiple times; if so, will be applied from highest to lowest index. Will be applied after all step updates from --update are applied.")
	flowsCmd.PersistentFlags().StringArrayVarP(&flagFlowStepAdds, "add", "a", nil, "Add a new step calling request REQ at index IDX, or at the end of current steps if index is omitted. Argument must be a string in form `[IDX]:REQ`. Can be given multiple times; if so, will be applied from lowest to highest index after all updates and removals are applied.")
	flowsCmd.PersistentFlags().StringArrayVarP(&flagFlowStepMoves, "move", "m", nil, "Move the step at index FROM to index TO. Argument must be a string in form `FROM:[TO]`. Can be given multiple times; if so, will be applied in order given after all replacements, removals, and adds are applied. If TO is not given, the step is moved to the end of the flow.")
	flowsCmd.PersistentFlags().StringArrayVarP(&flagFlowStepReplaces, "update", "u", nil, "Update the template called in step IDX to REQ. Argument must be a string in form `IDX:REQ`. Can be given multiple times; if so, will be applied in order given before any other step modifications.")
	flowsCmd.PersistentFlags().StringVarP(&flagFlowName, "name", "n", "", "Change the name of the flow to `NAME`.")

	flowsCmd.MarkFlagsMutuallyExclusive("delete", "new", "get", "remove")
	flowsCmd.MarkFlagsMutuallyExclusive("delete", "new", "get", "add")
	flowsCmd.MarkFlagsMutuallyExclusive("delete", "new", "get", "move")
	flowsCmd.MarkFlagsMutuallyExclusive("delete", "new", "get", "update")
	flowsCmd.MarkFlagsMutuallyExclusive("delete", "new", "get", "name")

	rootCmd.AddCommand(flowsCmd)
}

func invokeFlowsDelete(io cmdio.IO, projFile, flowName string) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	flowLower := strings.ToLower(flowName)
	if _, ok := p.Flows[flowLower]; !ok {
		return morc.NewFlowNotFoundError(flowName)
	}

	delete(p.Flows, flowLower)

	// save the project file
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Deleted flow %s\n", flowLower)

	return nil
}

func invokeFlowsEdit(io cmdio.IO, projFile, flowName string, attrs flowAttrValues) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	flowLower := strings.ToLower(flowName)
	flow, ok := p.Flows[flowLower]
	if !ok {
		return morc.NewFlowNotFoundError(flowName)
	}

	modifiedVals := map[flowKey]interface{}{}
	noChangeVals := map[flowKey]interface{}{}

	if attrs.name.set {
		newNameLower := strings.ToLower(attrs.name.v)
		if newNameLower != flowLower {
			if newNameLower == "" {
				return fmt.Errorf("new name cannot be empty")
			}
			if _, exists := p.Flows[newNameLower]; exists {
				return fmt.Errorf("flow named %s already exists", newNameLower)
			}

			flow.Name = newNameLower
			delete(p.Flows, flowLower)
			modifiedVals[flowKeyName] = newNameLower
		} else {
			noChangeVals[flowKeyName] = flowLower
		}
	}

	// build up order slice as we go to contain our values; prior arg parsing
	// must ensure args are actually in reasonable order.
	attrOrdering := make([]flowKey, len(flowAttrKeys))
	copy(attrOrdering, flowAttrKeys)
	stepOpCount := 0

	for _, upsert := range attrs.stepReplacements {
		idx := upsert.index
		newVal := strings.ToLower(upsert.template)

		var err error
		idx, err = flowStepIndexFromOrdinal(flow.Steps, idx, false)
		if err != nil {
			return fmt.Errorf("cannot set value of step #%d: %w", idx+1, err)
		}

		// no need for bounds check, already done in flowStepIndexFromOrdinal
		oldVal := strings.ToLower(flow.Steps[idx].Template)
		modKey := flowKey{stepIndex: idx + 1, uniqueInt: stepOpCount}
		stepOpCount++

		if oldVal != newVal {
			flow.Steps[idx].Template = newVal
			modifiedVals[modKey] = newVal
		} else {
			noChangeVals[modKey] = oldVal
		}
		attrOrdering = append(attrOrdering, modKey)
	}

	for _, delIdx := range attrs.stepRemovals {
		actualIdx, err := flowStepIndexFromOrdinal(flow.Steps, delIdx, false)
		if err != nil {
			return fmt.Errorf("cannot remove step #%d: %w", actualIdx, err)
		}

		removedTemplateName := flow.Steps[actualIdx].Template

		if err := flow.RemoveStep(actualIdx); err != nil {
			return fmt.Errorf("cannot remove step #%d: %w", actualIdx+1, err)
		}

		if removedTemplateName == "" {
			removedTemplateName = `""`
		}
		modKey := flowKey{stepIndex: actualIdx + 1, uniqueInt: stepOpCount}
		stepOpCount++
		modifiedVals[modKey] = fmt.Sprintf("no longer exist; was %s (removed)", removedTemplateName)
		attrOrdering = append(attrOrdering, modKey)
	}

	for _, add := range attrs.stepAdds {
		// apply step index conversion as if flows were one bigger to allow for
		// one-past end

		updatedSteps := make([]morc.FlowStep, len(flow.Steps)+1)
		actualIdx, err := flowStepIndexFromOrdinal(updatedSteps, add.index, true)
		if err != nil {
			return fmt.Errorf("cannot add step at #%d: %w", actualIdx, err)
		}

		// make shore the new template exists
		tmpl := strings.ToLower(add.template)
		if _, exists := p.Templates[tmpl]; !exists {
			return fmt.Errorf("no request template %q in project", add.template)
		}

		newStep := morc.FlowStep{
			Template: tmpl,
		}

		if err := flow.InsertStep(actualIdx, newStep); err != nil {
			return fmt.Errorf("cannot add step at #%d: %w", actualIdx+1, err)
		}

		tmplName := tmpl
		if tmplName == "" {
			tmplName = `""`
		}
		modKey := flowKey{stepIndex: actualIdx + 1, uniqueInt: stepOpCount}
		stepOpCount++
		modifiedVals[modKey] = fmt.Sprintf("%s (added)", tmplName)
		attrOrdering = append(attrOrdering, modKey)
	}

	for _, move := range attrs.stepMoves {
		actualFrom, err := flowStepIndexFromOrdinal(flow.Steps, move.from, false)
		if err != nil {
			return fmt.Errorf("cannot move step #%d: step %w", actualFrom, err)
		}
		actualTo, err := flowStepIndexFromOrdinal(flow.Steps, move.to, true)
		if err != nil {
			return fmt.Errorf("cannot move step #%d to #%d: destination %w", actualFrom+1, actualTo, err)
		}

		modKey := flowKey{stepIndex: actualFrom + 1, uniqueInt: stepOpCount}
		stepOpCount++

		if actualFrom != actualTo {
			if err := flow.MoveStep(actualFrom, actualTo); err != nil {
				return fmt.Errorf("cannot move step #%d to #%d: %w", actualFrom+1, actualTo+1, err)
			}

			// always assume that the move is valid
			modifiedVals[modKey] = fmt.Sprintf("position #%d", actualTo+1)
		} else {
			noChangeVals[modKey] = fmt.Sprintf("position #%d", actualFrom+1)
		}
		attrOrdering = append(attrOrdering, modKey)
	}

	// flow name might have been modified so take the currently set .Name and lowercase it.
	p.Flows[strings.ToLower(flow.Name)] = flow
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	cmdio.OutputLoudEditAttrsResult(io, modifiedVals, noChangeVals, attrOrdering)

	return nil
}

func invokeFlowsGet(io cmdio.IO, projFile, flowName string, getItem flowKey) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names

	flowName = strings.ToLower(flowName)
	flow, ok := p.Flows[flowName]
	if !ok {
		return morc.NewFlowNotFoundError(flowName)
	}

	switch getItem {
	case flowKeyName:
		io.Printf("%s\n", flow.Name)
	default:
		idx := getItem.stepIndex
		idx, err = flowStepIndexFromOrdinal(flow.Steps, idx, false)
		if err != nil {
			return fmt.Errorf("cannot get step #%d: %w", getItem.stepIndex, err)
		}

		if idx >= len(flow.Steps) {
			return fmt.Errorf("%d doesn't exist; highest step index in %s is %d", idx, flow.Name, len(flow.Steps)-1)
		}

		io.Printf("%s\n", flow.Steps[idx].Template)
	}

	return nil
}

func invokeFlowsNew(io cmdio.IO, projFile, flowName string, templates []string) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	flowLower := strings.ToLower(flowName)

	// check if the project already has a flow with the same name
	if _, exists := p.Flows[flowLower]; exists {
		return morc.NewFlowExistsError(flowName)
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
		Name:  flowName,
		Steps: steps,
	}

	if p.Flows == nil {
		p.Flows = map[string]morc.Flow{}
	}

	p.Flows[flowLower] = flow

	// save the project file
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Created new flow %s with %s\n", flowLower, io.CountOf(len(templates), "step"))

	return nil
}

func invokeFlowsShow(io cmdio.IO, projFile, flowName string) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for flow names
	flowLower := strings.ToLower(flowName)
	flow, ok := p.Flows[flowLower]
	if !ok {
		return morc.NewFlowNotFoundError(flowName)
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

func invokeFlowsList(io cmdio.IO, projFile string) error {
	p, err := morc.LoadProjectFromDisk(projFile, false)
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

type flowsArgs struct {
	projFile string
	action   flowAction
	getItem  flowKey
	flow     string
	reqs     []string
	sets     flowAttrValues
}

type flowAttrValues struct {
	name             optional[string]
	stepReplacements []flowStepUpsert
	stepAdds         []flowStepUpsert
	stepRemovals     []int
	stepMoves        []flowStepMove
}

type flowStepUpsert struct {
	index    int
	template string
}

type flowStepMove struct {
	from int
	to   int
}

func parseFlowsArgs(cmd *cobra.Command, posArgs []string, args *flowsArgs) error {
	args.projFile = commonflags.ProjectFile
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseFlowsActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	// do action-specific arg and flag parsing
	switch args.action {
	case flowsList:
		// nothing else to do
	case flowsShow:
		// set arg 1 as the flow name
		args.flow = posArgs[0]
	case flowsDelete:
		// special case of flow name set from a CLI flag rather than pos arg.
		args.flow = flagFlowDelete
	case flowsGet:
		// set arg 1 as the flow name
		args.flow = posArgs[0]

		// parse the get from the string
		args.getItem, err = parseFlowAttrKey(flagFlowGet)
		if err != nil {
			return err
		}
	case flowsNew:
		// pick up requests from args and set the flow name from the flag
		args.flow = flagFlowNew
		args.sets.name = optional[string]{set: true, v: flagFlowNew}
		args.reqs = posArgs
	case flowsEdit:
		// set arg 1 as the flow name
		args.flow = posArgs[0]

		if err := parseFlowsSetFlags(cmd, &args.sets); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unhandled flow action %q", args.action))
	}

	return nil
}

func parseFlowsActionFromFlags(cmd *cobra.Command, posArgs []string) (flowAction, error) {
	// Enforcements assumed:
	// * mut-exc enforced by cobra: --new and --get will not both be present.
	// * mut-exc enforced by cobra: --new and --delete will not both be present.
	// * mut-exc enforced by cobra: --get and --delete will not both be present.
	// * mut-exc enforced by cobra: --delete and setOpts will not both be
	// present.
	// * mut-exc enforced by cobra: --get and setOpts will not both be set
	// * mut-exc enforced by cobra: --new and setOpts will not both be set

	f := cmd.Flags()

	if f.Changed("delete") {
		if len(posArgs) > 0 {
			return flowsDelete, fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return flowsDelete, nil
	} else if f.Changed("new") {
		if len(posArgs) < 2 {
			return flowsNew, fmt.Errorf("--new requires at least two requests in positional args")
		}
		return flowsNew, nil
	} else if f.Changed("get") {
		if len(posArgs) < 1 {
			return flowsGet, fmt.Errorf("missing name of FLOW to get from")
		}
		if len(posArgs) > 1 {
			return flowsGet, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}
		return flowsGet, nil
	} else if flowsSetFlagIsPresent(cmd) {
		if len(posArgs) < 1 {
			return flowsEdit, fmt.Errorf("missing name of FLOW to update")
		}
		if len(posArgs) > 1 {
			return flowsEdit, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}
		return flowsEdit, nil
	}

	if len(posArgs) == 0 {
		return flowsList, nil
	} else if len(posArgs) == 1 {
		return flowsShow, nil
	} else {
		return flowsList, fmt.Errorf("unknown positional argument %q", posArgs[1])
	}
}

func parseFlowsSetFlags(cmd *cobra.Command, attrs *flowAttrValues) error {
	f := cmd.Flags()

	if f.Lookup("name").Changed {
		attrs.name = optional[string]{set: true, v: flagFlowName}
	}

	if f.Lookup("update").Changed {
		// replace is in form IDX:REQ, no exceptions.
		for flagIdx, repl := range flagFlowStepReplaces {
			up, err := parseFlowUpsertArg(repl, false)
			if err != nil {
				return fmt.Errorf("--update #%d: %w", flagIdx+1, err)
			}

			attrs.stepReplacements = append(attrs.stepReplacements, up)
		}
	}

	if f.Lookup("remove").Changed {
		// remove is in form IDX, no exceptions.
		attrs.stepRemovals = flagFlowStepRemovals
	}

	if f.Lookup("add").Changed {
		// add is in form IDX:REQ, optionally may be :REQ (or just REQ).
		for flagIdx, add := range flagFlowStepAdds {
			up, err := parseFlowUpsertArg(add, true)
			if err != nil {
				return fmt.Errorf("--add #%d: %w", flagIdx+1, err)
			}

			attrs.stepAdds = append(attrs.stepAdds, up)
		}
	}

	if f.Lookup("move").Changed {
		// move is in form FROM:TO, optionally may be FROM: (or just FROM).
		for flagIdx, move := range flagFlowStepMoves {
			up, err := parseFlowMoveArg(move)
			if err != nil {
				return fmt.Errorf("--move #%d: %w", flagIdx+1, err)
			}

			attrs.stepMoves = append(attrs.stepMoves, up)
		}
	}

	return nil
}

func parseFlowMoveArg(s string) (flowStepMove, error) {
	var move flowStepMove

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		if len(parts) == 1 {
			parts = strings.Split(s+":", ":")
		} else {
			return move, fmt.Errorf("not in FROM:TO, FROM:, or FROM format: %q", s)
		}
	}

	var err error

	move.from, err = strconv.Atoi(parts[0])
	if err != nil {
		return move, fmt.Errorf("FROM: not a valid step index: %q", parts[0])
	}

	if parts[1] == "" {
		move.to = -1
		// move to end of slice
	} else {
		move.to, err = strconv.Atoi(parts[1])
		if err != nil {
			return move, fmt.Errorf("TO: not a valid step index: %q", parts[1])
		}

		if move.to < 0 {
			move.to = -1
		}
	}

	return move, nil
}

func parseFlowUpsertArg(s string, optionalIndex bool) (flowStepUpsert, error) {
	var ups flowStepUpsert
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 1 || (len(parts) == 2 && parts[0] == "") {
		if optionalIndex {
			ups.index = -1
		} else {
			return ups, fmt.Errorf("not in IDX:REQ format: %q", s)
		}
	} else {
		var err error
		ups.index, err = strconv.Atoi(parts[0])
		if err != nil {
			return ups, fmt.Errorf("IDX %q is not an integer", parts[0])
		}
	}

	ups.template = strings.ToLower(parts[len(parts)-1])

	return ups, nil
}

func flowsSetFlagIsPresent(cmd *cobra.Command) bool {
	f := cmd.Flags()
	return f.Changed("add") || f.Changed("remove") || f.Changed("move") || f.Changed("update") || f.Changed("name")
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

	// uniqueInt is not used in display but is used for key uniqueness.
	// needed only when in printing output with multiple possible operations originally on the same index during an edit call
	uniqueInt int
}

func (fk flowKey) isStepIndex() bool {
	return fk.name == ""
}

var (
	flowKeyName flowKey = flowKey{name: "NAME"}
)

// Human prints the human-readable description of the key.
func (fk flowKey) Human() string {
	if fk.isStepIndex() {
		return fmt.Sprintf("step #%d", fk.stepIndex)
	}

	switch fk.name {
	case flowKeyName.name:
		return "flow name"
	default:
		return fmt.Sprintf("unknown flow key %q", fk.name)
	}
}

func (fk flowKey) Name() string {
	if fk.isStepIndex() {
		return fmt.Sprintf("%d", fk.stepIndex)
	} else {
		return string(fk.name)
	}
}

var (
	// ordering of flowAttrKeys in output is set here

	flowAttrKeys = []flowKey{
		flowKeyName,
	}
)

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
			return flowKey{}, fmt.Errorf("must be a step index or one of: %s", strings.Join(flowAttrKeyNames(), ", "))
		}

		return flowKey{stepIndex: idx}, nil
	}

	switch strings.ToUpper(s) {
	case flowKeyName.Name():
		return flowKeyName, nil
	default:
		return flowKey{}, fmt.Errorf("must be a step index or one of: %s", strings.Join(flowAttrKeyNames(), ", "))
	}
}

func flowStepIndexFromOrdinal(steps []morc.FlowStep, idx int, autoClampMax bool) (int, error) {
	// basically, we will decrement (unless the sigil value -1) and then feed to
	// sliceops func that will translate -1's. If the index cannot be translated
	// to a valid index, we will return an error.

	// TODO: horribly messy to refer to steps by number but actually store them by index. this tool is for engineers; we know it's 0-based.

	if idx == 0 {
		// never valid; no 0th value for ordinals
		return 0, fmt.Errorf("does not exist")
	}

	if idx != -1 {
		return sliceops.RealIndex(steps, idx-1, autoClampMax)
	}
	return sliceops.RealIndex(steps, idx, autoClampMax)
}
