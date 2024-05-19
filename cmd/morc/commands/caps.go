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
	flagCapsNew    bool
	flagCapsDelete bool
)

func init() {
	capsCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	capsCmd.PersistentFlags().BoolVarP(&flagCapsNew, "new", "", false, "Create a new variable capture on the request. If given, the specification of the new capture must also be given as a third argument.")
	capsCmd.PersistentFlags().BoolVarP(&flagCapsDelete, "delete", "d", false, "Delete the given variable capture from the request. Can only be used if giving REQ and CAP and no other arguments.")

	// cannot delete while doing new
	capsCmd.MarkFlagsMutuallyExclusive("new", "delete")

	rootCmd.AddCommand(capsCmd)
}

var capsCmd = &cobra.Command{
	Use: "caps REQ [-F project_file]\n" +
		"caps REQ CAP [-d] [-F project_file]\n" +
		"caps REQ CAP SPECIFICATION --new\n" +
		"caps REQ CAP ATTR [VALUE [ATTR2 VALUE2]...]",
	GroupID: projMetaCommands.ID,
	Short:   "Get or modify variable captures on a request template.",
	Long:    "Perform operations on variable captures defined on a request template. With only the name of the request template given, prints out a listing of all the captures defined on the given request. For all other operations, CAP must be specified; this is the name of the variable that the capture is saved to, and serves as the primary identifier for variable captures. If CAP is given with no other arguments, information on that capture is printed. If --new is given with CAP, a new capture will be created on the request that saves to the var called CAP and that captures data from responses with the given specification. If -d is given, var capture CAP is deleted from the request template. If a capture attribute name is given after CAP, only that particular attribute is printed out. If one or more pairs of capture attributes and new values are given, those attributes on CAP will be set to their corresponding values.\n\nCapture specifications can be given in one of two formats. They can be in format ':START,END' for a byte offset (ex: \":4,20\") or a jq-ish path with only keys and variable indexes (ex: \"records[1].auth.token\")",
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := capsOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		reqName := args[0]
		if reqName == "" {
			return fmt.Errorf("request name cannot be empty")
		}

		// parse args and decide action based on flags and number of args

		var varName string
		var varSpec string
		var getItem capKey

		// first ensure user isn't trying to use -d with anyfin other than a single CAP
		if flagCapsDelete {
			if len(args) < 2 {
				return fmt.Errorf("-d flag can only be used if REQ and CAP are given")
			}
			if len(args) > 2 {
				return fmt.Errorf("-d flag cannot be used with arguments other than REQ and CAP")
			}
		}
		// do the same check but for NEW, user must give only a spec
		if flagCapsNew {
			if len(args) < 3 {
				return fmt.Errorf("--new flag can only be used if REQ, CAP, and SPECIFICATION are given")
			}
			if len(args) > 3 {
				return fmt.Errorf("--new flag cannot be used with arguments other than REQ, CAP, and SPECIFICATION")
			}
		}

		// normal parsing and checking now
		if len(args) == 1 {
			// only one possible action: list
			opts.action = capsList
		} else if len(args) == 2 {
			// this is a show or a delete, depending on whether -d is set
			varName = args[1]

			if flagCapsDelete {
				opts.action = capsDelete
			} else {
				opts.action = capsShow
			}
		} else {
			varName = args[1]

			// either a new or a get item

			if flagCapsNew {
				opts.action = capsNew

				// var spec needs to be grabbed but save parsing for post-usage
				// printing error
				varSpec = args[2]
			} else {
				// full arg parsing mode
				var curKey capKey
				var err error

				for i, arg := range args[2:] {
					if i%2 == 0 {
						// if even, should be an attribute.
						curKey, err = parseCapAttrKey(arg)
						if err != nil {
							return fmt.Errorf("attribute #%d: %w", (i/2)+1, err)
						}

						// do an "already set" check
						setTwice := false
						switch curKey {
						case capKeyVar:
							setTwice = opts.capVar.set
						case capKeySpec:
							setTwice = opts.spec.set
						}

						if setTwice {
							return fmt.Errorf("%s is set more than once", curKey)
						}
					} else {
						// if odd, it is a value
						switch curKey {
						case capKeyVar:
							opts.capVar = optional[string]{set: true, v: arg}
						case capKeySpec:
							opts.spec = optional[string]{set: true, v: arg}
						}
					}
				}

				// now that we are done, do an arg-count check and use it to set
				// action.
				// doing AFTER parsing so that we can give a betta error message if
				// missing last value
				if len(args[2:]) == 1 {
					// that's fine, we just want to get the one item
					opts.action = capsGet
					getItem = curKey
				} else if len(args)%2 != 0 {
					return fmt.Errorf("%s is missing a value", curKey)
				} else {
					opts.action = capsEdit
				}
			}
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch opts.action {
		case capsList:
			return invokeCapsList(io, reqName, opts)
		case capsShow:
			return invokeCapsShow(io, reqName, varName, opts)
		case capsDelete:
			return invokeCapsDelete(io, reqName, varName, opts)
		case capsNew:
			return invokeCapsNew(io, reqName, varName, varSpec, opts)
		case capsGet:
			return invokeCapsGet(io, reqName, varName, getItem, opts)
		case capsEdit:
			return invokeCapsEdit(io, reqName, varName, opts)
		default:
			return fmt.Errorf("unknown action %d", opts.action)
		}
	},
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

type capsOptions struct {
	projFile string
	action   capsAction

	capVar optional[string]
	spec   optional[string]
}

func invokeCapsDelete(io cmdio.IO, reqName, varName string, opts capsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
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

func invokeCapsEdit(io cmdio.IO, reqName, varName string, opts capsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, false)
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
	if !opts.capVar.set && !opts.spec.set {
		return fmt.Errorf("no changes requested")
	}

	modifiedVals := map[capKey]interface{}{}
	noChangeVals := map[capKey]interface{}{}

	// if we have a name change, apply that first
	if opts.capVar.set {
		newNameUpper := strings.ToUpper(opts.capVar.v)

		// if new name same as old, no reason to do additional work
		if newNameUpper != varUpper {
			// check if the new name is already in use
			if _, ok := req.Captures[newNameUpper]; ok {
				return fmt.Errorf("capture to variable $%s already exists in request %s", opts.capVar.v, reqName)
			}

			// remove the old name
			delete(req.Captures, varUpper)

			// add the new one; we will update the name when we save it back to
			// the project
			cap.Name = opts.capVar.v

			modifiedVals[capKeyVar] = opts.capVar.v
		} else {
			noChangeVals[capKeyVar] = varUpper
		}
	}

	// if we have a spec change, apply that next
	if opts.spec.set {
		newCap, err := morc.ParseVarScraperSpec(cap.Name, opts.spec.v)
		if err != nil {
			return fmt.Errorf("spec: %w", err)
		}

		if !cap.EqualSpec(newCap) {
			cap = newCap
			modifiedVals[capKeySpec] = opts.spec.v
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

func invokeCapsGet(io cmdio.IO, reqName, capName string, getItem capKey, opts capsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
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

func invokeCapsNew(io cmdio.IO, reqName, varName, varCap string, opts capsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqName = strings.ToLower(reqName)
	req, ok := p.Templates[reqName]
	if !ok {
		return fmt.Errorf("no request template %s", reqName)
	}

	// parse the var scraper
	varName, err = morc.ParseVarName(varName)
	if err != nil {
		return err
	}

	// var name normalized to upper case
	varUpper := strings.ToUpper(varName)
	if len(req.Captures) > 0 {
		if _, ok := req.Captures[varUpper]; ok {
			return fmt.Errorf("variable $%s already has a capture", varUpper)
		}
	}

	// parse the capture spec
	cap, err := morc.ParseVarScraperSpec(varName, varCap)
	if err != nil {
		return err
	}

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

func invokeCapsList(io cmdio.IO, reqName string, opts capsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
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

func invokeCapsShow(io cmdio.IO, reqName, capName string, opts capsOptions) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
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
