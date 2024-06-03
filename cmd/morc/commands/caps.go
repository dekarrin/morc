package commands

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagCapsNew    string
	flagCapsDelete string
	flagCapsGet    string
	flagCapsSpec   string
	flagCapsVar    string
)

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

		switch args.action {
		case capsList:
			return invokeCapsList(io, args.projFile, args.request)
		case capsShow:
			return invokeCapsShow(io, args.projFile, args.request, args.capture)
		case capsDelete:
			return invokeCapsDelete(io, args.projFile, args.request, args.capture)
		case capsNew:
			return invokeCapsNew(io, args.projFile, args.request, args.capture, args.sets)
		case capsGet:
			return invokeCapsGet(io, args.projFile, args.request, args.capture, args.getItem)
		case capsEdit:
			return invokeCapsEdit(io, args.projFile, args.request, args.capture, args.sets)
		default:
			return fmt.Errorf("unknown action %d", args.action)
		}
	},
}

func init() {
	capsCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	capsCmd.PersistentFlags().StringVarP(&flagCapsNew, "new", "N", "", "Create a new capture on REQ that saves captured data to `VAR`. If given, the specification of the new capture must also be given with --spec/-s.")
	capsCmd.PersistentFlags().StringVarP(&flagCapsDelete, "delete", "D", "", "Delete the given variable capture `VAR` from the request.")
	capsCmd.PersistentFlags().StringVarP(&flagCapsGet, "get", "G", "", "Get the value of a specific attribute `ATTR` of the capture. Can only be used if giving REQ and CAP and no other arguments.")
	capsCmd.PersistentFlags().StringVarP(&flagCapsSpec, "spec", "s", "", "Specify where in responses that data should be captured from. `SPEC` is a specially-formatted string of form :FROM,TO to specify a byte-offset or a jq-ish syntax string to specify a path to a value within a JSON response body.")
	capsCmd.PersistentFlags().StringVarP(&flagCapsVar, "var", "V", "", "Set the variable that the capture saves to.")

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
	p, err := morc.LoadProjectFromDisk(projFile, false)
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
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Deleted capture to %s from %s", varUpper, reqName)

	return nil
}

func invokeCapsEdit(io cmdio.IO, projFile, reqName, varName string, attrs capAttrValues) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, false)
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
		return fmt.Errorf("no capture to variable $%s exists in request %s", varUpper, reqName)
	}
	cap, ok := req.Captures[varUpper]
	if !ok {
		return fmt.Errorf("no capture to variable $%s exists in request %s", varUpper, reqName)
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
				return fmt.Errorf("capture to variable $%s already exists in request %s", attrs.capVar.v, reqName)
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
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	cmdio.OutputLoudEditAttrsResult(io, modifiedVals, noChangeVals, capAttrKeys)

	return nil
}

func invokeCapsGet(io cmdio.IO, projFile, reqName, capName string, getItem capKey) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, true)
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
	p, err := morc.LoadProjectFromDisk(projFile, true)
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
			return fmt.Errorf("variable $%s already has a capture", varUpper)
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
	err = p.PersistToDisk(false)
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
	p, err := morc.LoadProjectFromDisk(projFile, true)
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
		for _, cap := range req.Captures {
			io.Printf("%s\n", cap)
		}
	}

	return nil
}

func invokeCapsShow(io cmdio.IO, projFile, reqName, capName string) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, true)
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

	fmt.Printf("%s\n", cap.String())

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
	args.projFile = commonflags.ProjectFile
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
	case capsList:
		// nothing else to do; all args already gathered
	case capsShow:
		// set arg 2 as the capture name
		args.capture = posArgs[1]
	case capsDelete:
		// special case of capture set from a CLI flag rather than pos arg.
		args.capture = flagCapsDelete
	case capsGet:
		// set arg 2 as the capture name
		args.capture = posArgs[1]

		// parse the get from the string
		args.getItem, err = parseCapAttrKey(flagProjGet)
		if err != nil {
			return err
		}
	case capsNew:
		// above parsing already checked that -V will not be present and -s will
		// so we can just run through normal parseCapsSetFlags and then use
		// --new argument to set the new cap var manaully.
		if err := parseCapsSetFlags(cmd, &args.sets); err != nil {
			return err
		}

		// still need to parse the new name, above func won't hit it
		name, err := morc.ParseVarName(flagCapsVar)
		if err != nil {
			return fmt.Errorf("--new/-N: %w", err)
		}

		// apply it to both the sets and the capture name. callers SHOULD only
		// grab from args.capture, but this way is a bit more defensive.
		args.sets.capVar = optional[string]{set: true, v: name}
		args.capture = name
	case capsEdit:
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

	if flagCapsDelete != "" {
		if len(posArgs) < 1 {
			return capsDelete, fmt.Errorf("missing request REQ to delete capture from")
		}
		if len(posArgs) > 1 {
			return capsDelete, fmt.Errorf("unknown 2nd positional argument: %q", posArgs[1])
		}
		return capsDelete, nil
	} else if flagCapsNew != "" {
		if len(posArgs) < 1 {
			return capsNew, fmt.Errorf("missing request REQ to add new capture to")
		}
		if len(posArgs) > 1 {
			return capsNew, fmt.Errorf("unknown 2nd positional argument: %q", posArgs[1])
		}
		if !cmd.Flags().Changed("spec") {
			return capsNew, fmt.Errorf("--new/-N requires --spec/-s")
		}
		if cmd.Flags().Changed("var") {
			return capsNew, fmt.Errorf("--new/-N already gives var name; cannot be used with --var/-V")
		}
		return capsNew, nil
	} else if flagCapsGet != "" {
		if len(posArgs) < 1 {
			return capsGet, fmt.Errorf("missing request REQ and capture VAR to get attribute from")
		}
		if len(posArgs) < 2 {
			return capsGet, fmt.Errorf("missing capture VAR to get attribute from")
		}
		if len(posArgs) > 2 {
			return capsGet, fmt.Errorf("unknown 3rd positional argument: %q", posArgs[2])
		}
		return capsGet, nil
	} else if capsSetFlagIsPresent() {
		if len(posArgs) < 1 {
			return capsEdit, fmt.Errorf("missing request REQ and capture VAR to edit")
		}
		if len(posArgs) < 2 {
			return capsEdit, fmt.Errorf("missing capture var to edit")
		}
		if len(posArgs) > 2 {
			return capsEdit, fmt.Errorf("unknown 3rd positional argument: %q", posArgs[2])
		}
		return capsEdit, nil
	}

	if len(posArgs) == 0 {
		return capsList, fmt.Errorf("missing request REQ to list captures for")
	} else if len(posArgs) == 1 {
		return capsList, nil
	} else if len(posArgs) == 2 {
		return capsShow, nil
	} else {
		return capsShow, fmt.Errorf("unknown 3rd positional argument: %q", posArgs[2])
	}
}

func parseCapsSetFlags(cmd *cobra.Command, attrs *capAttrValues) error {
	if cmd.Flags().Lookup("spec").Changed {
		spec, err := morc.ParseVarScraperSpec("", flagCapsSpec)
		if err != nil {
			return fmt.Errorf("--spec/-s: %w", err)
		}

		attrs.spec = optional[morc.VarScraper]{set: true, v: spec}
	}

	if cmd.Flags().Lookup("var").Changed {
		name, err := morc.ParseVarName(flagCapsVar)
		if err != nil {
			return fmt.Errorf("--var/-V: %w", err)
		}
		attrs.capVar = optional[string]{set: true, v: name}
	}

	return nil
}

func capsSetFlagIsPresent() bool {
	return flagCapsVar != "" || flagCapsSpec != ""
}

type capsAction int

const (
	capsList capsAction = iota
	capsShow
	capsGet
	capsDelete
	capsNew
	capsEdit
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
