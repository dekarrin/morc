package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

// TODO: consistency between giving --new or -N, --delete or -D, --get or -G, across all commands
var capsCmd = &cobra.Command{
	Use: "caps REQ [VAR]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"caps REQ\n" +
			"caps REQ --delete VAR\n" +
			"caps REQ --new VAR -s SPEC\n" +
			"caps REQ VAR\n" +
			"caps REQ VAR --get ATTR\n" +
			"caps REQ VAR [-sV]",
	},
	GroupID: projMetaCommands.ID,
	Short:   "Get or modify variable captures on a request template.",
	Long: "Perform operations on variable captures defined on a request template. With only the name REQ of the request " +
		"template given, prints out a listing of all the captures defined on the request.\n\n" +
		"To create a new capture, provide --new with the name of the variable to capture to as its argument. " +
		"Additionally, the -s/--spec flag must be given to provide the location within responses that the variable's " +
		"value is to be taken from.\n\n" +
		"A capture can be viewed by providing VAR, the name of the variable that the capture saves to. To view only a " +
		"single attribute of a capture, give VAR as an argument and provide --get along with the name of the " +
		"attribute to view. The available names are: " + strings.Join(capAttrKeyNames(), ", ") + ".\n\n" +
		"To modify a capture, use one of the -s or -V flags when giving the VAR of the capture; -s will alter the spec, " +
		"and -V will change the captured-to variable.\n\n" +
		"A capture is removed from a request by providing --delete and the VAR of the capture to be deleted.\n\n" +
		"Capture specifications can be given in one of two formats. They can be in format ':START,END' for a byte " +
		"offset (ex: \":4,20\") or a jq-ish path with only keys and array indexes (ex: \"records[1].auth.token\")",
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args capsArgs
		if err := parseCapsArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		io.Quiet = flags.BQuiet

		switch args.action {
		case capsActionList:
			return invokeCapsList(io, args.projFile, args.request)
		case capsActionShow:
			return invokeCapsShow(io, args.projFile, args.request, args.capture)
		case capsActionDelete:
			return invokeCapsDelete(io, args.projFile, args.request, args.capture)
		case capsActionNew:
			return invokeCapsNew(io, args.projFile, args.request, args.capture, args.sets)
		case capsActionGet:
			return invokeCapsGet(io, args.projFile, args.request, args.capture, args.getItem)
		case capsActionEdit:
			return invokeCapsEdit(io, args.projFile, args.request, args.capture, args.sets)
		default:
			return fmt.Errorf("unknown action %d", args.action)
		}
	},
}

func init() {
	capsCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	capsCmd.PersistentFlags().StringVarP(&flags.New, "new", "N", "", "Create a new capture on REQ that saves captured data to `VAR`. If given, the specification of the new capture must also be given with --spec/-s.")
	capsCmd.PersistentFlags().StringVarP(&flags.Delete, "delete", "D", "", "Delete the given variable capture `VAR` from the request.")
	capsCmd.PersistentFlags().StringVarP(&flags.Get, "get", "G", "", "Get the value of a specific attribute `ATTR` of the capture. Can only be used if giving REQ and CAP and no other arguments.")
	capsCmd.PersistentFlags().StringVarP(&flags.Spec, "spec", "s", "", "Specify where in responses that data should be captured from. `SPEC` is a specially-formatted string of form :FROM,TO to specify a byte-offset or a jq-ish syntax string to specify a path to a value within a JSON response body.")
	capsCmd.PersistentFlags().StringVarP(&flags.VarName, "var", "V", "", "Set the variable that the capture saves to to `VAR`.")
	capsCmd.PersistentFlags().BoolVarP(&flags.BQuiet, "quiet", "q", false, "Suppress all unnecessary output.")

	// cannot delete while doing new
	capsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get")
	capsCmd.MarkFlagsMutuallyExclusive("delete", "spec")
	capsCmd.MarkFlagsMutuallyExclusive("delete", "var")
	capsCmd.MarkFlagsMutuallyExclusive("get", "spec")
	capsCmd.MarkFlagsMutuallyExclusive("get", "var")
	capsCmd.MarkFlagsMutuallyExclusive("new", "var")

	rootCmd.AddCommand(capsCmd)
}

func invokeCapsDelete(io cmdio.IO, projFile string, reqName, varName string) error {
	// load the project file
	p, err := readProject(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)
	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	// var name normalized to upper case
	varUpper := strings.ToUpper(varName)
	if len(req.Captures) > 0 {
		if _, ok := req.Captures[varUpper]; !ok {
			// TODO: standardize "not-found" error messages
			return fmt.Errorf("no capture defined for %s in %s", reqName, varUpper)
		}
	}

	// remove the capture
	delete(req.Captures, varUpper)
	p.Templates[reqName] = req

	// save the project file
	err = writeProject(p, false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Deleted capture to %s from %s", varUpper, reqName)

	return nil
}

func invokeCapsEdit(io cmdio.IO, projFile, reqName, varName string, attrs capAttrValues) error {
	// load the project file
	p, err := readProject(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)
	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	// case doesn't matter for var names
	varUpper := strings.ToUpper(varName)
	if len(req.Captures) == 0 {
		return fmt.Errorf("no capture to variable %s%s exists in request %s", p.VarPrefix(), varUpper, reqName)
	}
	cap, ok := req.Captures[varUpper]
	if !ok {
		return fmt.Errorf("no capture to variable %s%s exists in request %s", p.VarPrefix(), varUpper, reqName)
	}

	// okay did the user actually ask to change somefin
	if !attrs.capVar.set && !attrs.spec.set {
		return fmt.Errorf("no changes requested")
	}

	modifiedVals := map[capKey]interface{}{}
	noChangeVals := map[capKey]interface{}{}

	// if we have a name change, apply that first
	if attrs.capVar.set {
		newNameUpper := strings.ToUpper(attrs.capVar.v)

		// if new name same as old, no reason to do additional work
		if newNameUpper != varUpper {
			// check if the new name is already in use
			if _, ok := req.Captures[newNameUpper]; ok {
				return fmt.Errorf("capture to variable %s%s already exists in request %s", p.VarPrefix(), attrs.capVar.v, reqName)
			}

			// remove the old name
			delete(req.Captures, varUpper)

			// add the new one; we will update the name when we save it back to
			// the project
			cap.Name = attrs.capVar.v

			modifiedVals[capKeyVar] = attrs.capVar.v
		} else {
			noChangeVals[capKeyVar] = varUpper
		}
	}

	// if we have a spec change, apply that next
	if attrs.spec.set {
		if !cap.EqualSpec(attrs.spec.v) {
			cap = attrs.spec.v
			modifiedVals[capKeySpec] = attrs.spec.v.Spec()
		} else {
			noChangeVals[capKeySpec] = cap.Spec()
		}
	}

	// update the request
	req.Captures[strings.ToUpper(cap.Name)] = cap
	p.Templates[reqName] = req

	// save the project file
	err = writeProject(p, false)
	if err != nil {
		return err
	}

	cmdio.OutputLoudEditAttrsResult(io, modifiedVals, noChangeVals, capAttrKeys)

	return nil
}

func invokeCapsGet(io cmdio.IO, projFile, reqName, capName string, getItem capKey) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)
	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	capName = strings.ToUpper(capName)
	cap, ok := req.Captures[capName]
	if !ok {
		return fmt.Errorf("no capture to %s exists on request template %s", capName, reqName)
	}

	switch getItem {
	case capKeyVar:
		io.Printf("%s\n", cap.Name)
	case capKeySpec:
		io.Printf("%s\n", cap.Spec())
	default:
		return fmt.Errorf("unknown item %q", getItem)
	}

	return nil
}

// additional constraints: attrs.spec must be set to valid spec or this will panic.
// arguably this means we should not be accepting the attrs as its the only other
// property at this time, but doing it for consistency and extensibility.
func invokeCapsNew(io cmdio.IO, projFile, reqName, varName string, attrs capAttrValues) error {
	if !attrs.spec.set {
		panic("invokeCapsNew called without spec attribute set")
	}

	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)
	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	// parsing already done; just coerce case
	varUpper := strings.ToUpper(varName)
	if len(req.Captures) > 0 {
		if _, ok := req.Captures[varUpper]; ok {
			return fmt.Errorf("variable %s%s already has a capture", p.VarPrefix(), varUpper)
		}
	}

	cap := attrs.spec.v
	cap.Name = varUpper

	// otherwise, we have a valid capture, so add it to the request.
	if req.Captures == nil {
		req.Captures = make(map[string]morc.VarScraper)
		p.Templates[reqName] = req
	}
	req.Captures[varUpper] = cap

	// save the project file
	err = writeProject(p, false)
	if err != nil {
		return err
	}

	scrapeSource := "response"
	if cap.IsJSONSpec() {
		scrapeSource = "JSON response body"
	} else if cap.IsOffsetSpec() {
		scrapeSource = "byte offset in response"
	}

	io.PrintLoudf("Added new capture from %s to %s on %s", scrapeSource, varUpper, reqName)

	return nil
}

func invokeCapsList(io cmdio.IO, projFile string, reqName string) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)

	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	if len(req.Captures) == 0 {
		io.PrintLoudln("(none)")
	} else {
		// sort the output for consistency
		sortedKeys := make([]string, 0, len(req.Captures))
		for key := range req.Captures {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)

		for _, capName := range sortedKeys {
			cap := req.Captures[capName]
			io.Printf("%s\n", cap)
		}
	}

	return nil
}

func invokeCapsShow(io cmdio.IO, projFile, reqName, capName string) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)
	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	capName = strings.ToUpper(capName)
	cap, ok := req.Captures[capName]
	if !ok {
		return fmt.Errorf("no capture to %s exists on request template %s", capName, reqName)
	}

	fmt.Printf("%s%s\n", p.VarPrefix(), cap.String())

	io.PrintLoudln(cap)
	return nil
}

type capsArgs struct {
	projFile string
	action   capsAction
	request  string
	capture  string
	getItem  capKey
	sets     capAttrValues
}

type capAttrValues struct {
	capVar optional[string]
	spec   optional[morc.VarScraper]
}

func parseCapsArgs(cmd *cobra.Command, posArgs []string, args *capsArgs) error {
	args.projFile = projPathFromFlagsOrFile(cmd)
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseCapsActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	// assume arg 1 exists and be the request name (already enforced by parse func above)
	args.request = posArgs[0]

	// do action-specific arg and flag parsing
	switch args.action {
	case capsActionList:
		// nothing else to do; all args already gathered
	case capsActionShow:
		// set arg 2 as the capture name
		args.capture = posArgs[1]
	case capsActionDelete:
		// special case of capture set from a CLI flag rather than pos arg.
		args.capture = flags.Delete
	case capsActionGet:
		// set arg 2 as the capture name
		args.capture = posArgs[1]

		// parse the get from the string
		args.getItem, err = parseCapAttrKey(flags.Get)
		if err != nil {
			return err
		}
	case capsActionNew:
		// above parsing already checked that -V will not be present and -s will
		// so we can just run through normal parseCapsSetFlags and then use
		// --new argument to set the new cap var manaully.
		if err := parseCapsSetFlags(cmd, &args.sets); err != nil {
			return err
		}

		// still need to parse the new name, above func won't hit it
		name, err := morc.ParseVarName(flags.New)
		if err != nil {
			return fmt.Errorf("--new/-N: %w", err)
		}

		// apply it to both the sets and the capture name. callers SHOULD only
		// grab from args.capture, but this way is a bit more defensive.
		args.sets.capVar = optional[string]{set: true, v: name}
		args.capture = name
	case capsActionEdit:
		// set arg 2 as the capture name
		args.capture = posArgs[1]

		if err := parseCapsSetFlags(cmd, &args.sets); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unhandled caps action %q", args.action))
	}

	return nil
}

func parseCapsActionFromFlags(cmd *cobra.Command, posArgs []string) (capsAction, error) {
	// Enforcements assumed:
	// * mut-exc enforced by cobra: --new and --get will not both be present.
	// * mut-exc enforced by cobra: --new and --delete will not both be present.
	// * mut-exc enforced by cobra: --get and --delete will not both be present.
	// * mut-exc enforced by cobra: --delete and setOpts will not both be
	// present.
	// * mut-exc enforced by cobra: --get and setOpts will not both be set
	// * mut-exc enforced by cobra: --new and --var setOpt will not be set
	// * Min args 1.

	if flags.Delete != "" {
		if len(posArgs) < 1 {
			return capsActionDelete, fmt.Errorf("missing request REQ to delete capture from")
		}
		if len(posArgs) > 1 {
			return capsActionDelete, fmt.Errorf("unknown 2nd positional argument: %q", posArgs[1])
		}
		return capsActionDelete, nil
	} else if flags.New != "" {
		if len(posArgs) < 1 {
			return capsActionNew, fmt.Errorf("missing request REQ to add new capture to")
		}
		if len(posArgs) > 1 {
			return capsActionNew, fmt.Errorf("unknown 2nd positional argument: %q", posArgs[1])
		}
		if !cmd.Flags().Changed("spec") {
			return capsActionNew, fmt.Errorf("--new/-N requires --spec/-s")
		}
		if cmd.Flags().Changed("var") {
			return capsActionNew, fmt.Errorf("--new/-N already gives var name; cannot be used with --var/-V")
		}
		return capsActionNew, nil
	} else if flags.Get != "" {
		if len(posArgs) < 1 {
			return capsActionGet, fmt.Errorf("missing request REQ and capture VAR to get attribute from")
		}
		if len(posArgs) < 2 {
			return capsActionGet, fmt.Errorf("missing capture VAR to get attribute from")
		}
		if len(posArgs) > 2 {
			return capsActionGet, fmt.Errorf("unknown 3rd positional argument: %q", posArgs[2])
		}
		return capsActionGet, nil
	} else if capsSetFlagIsPresent() {
		if len(posArgs) < 1 {
			return capsActionEdit, fmt.Errorf("missing request REQ and capture VAR to edit")
		}
		if len(posArgs) < 2 {
			return capsActionEdit, fmt.Errorf("missing capture var to edit")
		}
		if len(posArgs) > 2 {
			return capsActionEdit, fmt.Errorf("unknown 3rd positional argument: %q", posArgs[2])
		}
		return capsActionEdit, nil
	}

	if len(posArgs) == 0 {
		return capsActionList, fmt.Errorf("missing request REQ to list captures for")
	} else if len(posArgs) == 1 {
		return capsActionList, nil
	} else if len(posArgs) == 2 {
		return capsActionShow, nil
	} else {
		return capsActionShow, fmt.Errorf("unknown 3rd positional argument: %q", posArgs[2])
	}
}

func parseCapsSetFlags(cmd *cobra.Command, attrs *capAttrValues) error {
	if cmd.Flags().Lookup("spec").Changed {
		spec, err := morc.ParseVarScraperSpec("", flags.Spec)
		if err != nil {
			return fmt.Errorf("--spec/-s: %w", err)
		}

		attrs.spec = optional[morc.VarScraper]{set: true, v: spec}
	}

	if cmd.Flags().Lookup("var").Changed {
		name, err := morc.ParseVarName(flags.VarName)
		if err != nil {
			return fmt.Errorf("--var/-V: %w", err)
		}
		attrs.capVar = optional[string]{set: true, v: name}
	}

	return nil
}

func capsSetFlagIsPresent() bool {
	return flags.VarName != "" || flags.Spec != ""
}

type capsAction int

const (
	capsActionList capsAction = iota
	capsActionShow
	capsActionGet
	capsActionDelete
	capsActionNew
	capsActionEdit
)

type capKey string

const (
	capKeyVar  capKey = "VAR"
	capKeySpec capKey = "SPEC"
)

// Human prints the human-readable description of the key.
func (ck capKey) Human() string {
	switch ck {
	case capKeyVar:
		return "captured-to variable"
	case capKeySpec:
		return "capture specification"
	default:
		return fmt.Sprintf("unknown capture key %q", ck)
	}
}

func (ck capKey) Name() string {
	return string(ck)
}

var (
	// ordering of capKeys in output is set here

	capAttrKeys = []capKey{
		capKeyVar,
		capKeySpec,
	}
)

func capAttrKeyNames() []string {
	names := make([]string, len(capAttrKeys))
	for i, k := range capAttrKeys {
		names[i] = k.Name()
	}
	return names
}

func parseCapAttrKey(s string) (capKey, error) {
	switch strings.ToUpper(s) {
	case capKeyVar.Name():
		return capKeyVar, nil
	case capKeySpec.Name():
		return capKeySpec, nil
	default:
		return "", fmt.Errorf("invalid attribute %q; must be one of %s", s, strings.Join(capAttrKeyNames(), ", "))
	}
}
