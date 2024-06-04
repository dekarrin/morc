package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cliflags"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var (
	flagReqsBodyData string
	flagReqsHeaders  []string
	flagReqsMethod   string
	flagReqsURL      string
)

var reqsCmd = &cobra.Command{
	Use: "reqs [REQ]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"reqs\n" +
			"reqs --delete REQ [-f]\n" +
			"reqs --new REQ [-d DATA | -d @FILE] [-XuH]...\n" +
			"reqs REQ\n" +
			"reqs REQ --get ATTR\n" +
			"reqs REQ [-ndXuHrR]...",
	},
	GroupID: "project",
	Short:   "Show or modify request templates",
	Long: "Manipulate project request templates. By itself, prints out a listing of the names and methods of the " +
		"request templates in the project.\n\n" +
		"A new request template can be created by providing the name of it to the --new flag and using flags to " +
		"specify attributes to set on the new request. The method of the request is set with the --method/-X flag. " +
		"The payload in the request body is set with the -d/--data flag, either directly by providing the body as the " +
		"argument or indirectly by loading from a filename given after a leading '@'. Headers are set with the " +
		"-H/--header flag. Multiple headers may be specified by providing multiple -H flags. The URL of the request " +
		"is set with the the -u/--url flag.\n\n" +
		"A particular request can be viewed by providing the name of the request, REQ, as a positional argument to " +
		"the flows command. This will show all details of a request template. To see only a specific attribute of a " +
		"request, provide --get along with the name of the attribute of the request to show. The attribute, ATTR, " +
		"must be one of the following: " + strings.Join(reqAttrKeyNames(), ", ") + ". If 'HEADERS' is selected, all " +
		"headers on the request are printed. To see the value(s) of only a particular header, use --get-header with " +
		"the name of the header to see instead.\n\n" +
		"Modifications to existing request templates are performed by giving REQ as a positional argument followed by " +
		"one or more flag that sets a property of the request. For example, to change the method of a request, " +
		"provide the -X flag followed by the new method. All flags that are supported during request creation are " +
		"also supported when modifying a request (-X, -d, -u, -H), in addition to a few others. The name of the " +
		"request is updated with -n/--name. Since -H only *adds* new header values, --remove-header/-r can be used to " +
		"remove an existing header from the request. If it is a multi-valued header, only the last value added is " +
		"removed. Finally, calling --remove-body/-R will remove the body payload entirely, which may differ from " +
		"simply setting it to the empty string.\n\n" +
		"Requests are deleted by passing the --delete flag with a request name as its argument. This will " +
		"irreversibly remove the request from the project entirely.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args reqsArgs
		if err := parseReqsArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch args.action {
		case reqsList:
			return invokeReqsList(io, args.projFile)
		case reqsShow:
			return invokeReqsShow(io, args.projFile, args.req)
		case reqsDelete:
			return invokeReqsDelete(io, args.projFile, args.req, args.force)
		case reqsGet:
			return invokeReqsGet(io, args.projFile, args.req, args.getItem)
		case reqsNew:
			return invokeReqsNew(io, args.projFile, args.req, args.sets)
		case reqsEdit:
			return invokeReqsEdit(io, args.projFile, args.req, args.sets)
		default:
			panic(fmt.Sprintf("unhandled reqs action %q", args.action))
		}
	},
}

func init() {
	reqsCmd.PersistentFlags().StringVarP(&cliflags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	reqsCmd.PersistentFlags().StringVarP(&cliflags.New, "new", "N", "", "Create a new request template named `REQ`.")
	reqsCmd.PersistentFlags().StringVarP(&cliflags.Delete, "delete", "D", "", "Delete the request template named `REQ`.")
	reqsCmd.PersistentFlags().StringVarP(&cliflags.Get, "get", "G", "", "Get the value of the given attribute `ATTR` from the request. To get a particular header's value, use --get-header instead. ATTR must be one of: "+strings.Join(reqAttrKeyNames(), ", "))
	reqsCmd.PersistentFlags().StringVarP(&cliflags.GetHeader, "get-header", "", "", "Get the value(s) of the given header `KEY` that is currently set on the request.")
	reqsCmd.PersistentFlags().StringVarP(&cliflags.Name, "name", "n", "", "Change the name of a request template to `NAME`.")
	reqsCmd.PersistentFlags().StringArrayVarP(&cliflags.RemoveHeaders, "remove-header", "r", []string{}, "Remove header with key `KEY` from the request. If multiple headers with the same key exist, only the most recently added one will be deleted.")
	reqsCmd.PersistentFlags().StringVarP(&flagReqsBodyData, "data", "d", "", "Add the given `DATA` as a body to the request; prefix with '@' to instead interperet DATA as a filename that body data is to be read from.")
	reqsCmd.PersistentFlags().StringArrayVarP(&flagReqsHeaders, "header", "H", []string{}, "Add a header to the request. Format is `KEY:VALUE`. Multiple headers may be set by providing multiple -H flags. If multiple headers with the same key are set, they will be set in the order they were given.")
	reqsCmd.PersistentFlags().StringVarP(&flagReqsMethod, "method", "X", "GET", "Set the request method to `METHOD`.")
	reqsCmd.PersistentFlags().StringVarP(&flagReqsURL, "url", "u", "http://example.com", "Specify the `URL` for the request.")
	reqsCmd.PersistentFlags().BoolVarP(&cliflags.BRemoveBody, "remove-body", "R", false, "Delete all existing body data from the request")
	reqsCmd.PersistentFlags().BoolVarP(&cliflags.BForce, "force", "f", false, "Force deletion of the request template even if it is used in flows. Only valid with --delete/-D.")

	reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "name")
	reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "remove-header")
	reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "remove-body")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "data")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "method")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "header")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "url")
	reqsCmd.MarkFlagsMutuallyExclusive("data", "remove-body")
	reqsCmd.MarkFlagsMutuallyExclusive("new", "get", "get-header", "force")

	rootCmd.AddCommand(reqsCmd)
}

func invokeReqsEdit(io cmdio.IO, projFile, reqName string, attrs reqAttrValues) error {
	// first, are we updating the name? this determines whether we need to load
	// and save history
	loadAllFiles := attrs.name.set

	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, loadAllFiles)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqLower := strings.ToLower(reqName)
	req, ok := p.Templates[reqLower]
	if !ok {
		return morc.NewReqNotFoundError(reqLower)
	}

	modifiedVals := map[reqKey]interface{}{}
	noChangeVals := map[reqKey]interface{}{}

	// build up order slice as we go to contain our non-predefined values
	attrOrdering := make([]reqKey, len(reqAttrKeys))
	copy(attrOrdering, reqAttrKeys)
	nonPredefinedAttrCount := 0

	// if changing names, do that first
	if attrs.name.set {
		newName := strings.ToLower(attrs.name.v)

		if newName != reqLower {
			if _, exists := p.Templates[newName]; exists {
				return morc.NewReqExistsError(newName)
			}

			// update the name in the history
			for idx, h := range p.History {
				if strings.ToLower(h.Template) == reqLower {
					p.History[idx].Template = newName
				}
			}

			// update the name in the flows
			for flowName, flow := range p.Flows {
				for idx, step := range flow.Steps {
					if strings.ToLower(step.Template) == reqLower {
						p.Flows[flowName].Steps[idx].Template = newName
					}
				}
			}

			// update the name in the project
			req.Name = newName
			delete(p.Templates, reqLower)
			modifiedVals[reqKeyName] = newName
		} else {
			noChangeVals[reqKeyName] = newName
		}
	}

	// any name changes will have gone to req.Name at this point; be shore to
	// use that for any name displaying from this point forward

	// body modifications
	if attrs.body.set {
		if !(attrs.body.v == nil && req.Body == nil) {
			req.Body = attrs.body.v

			if req.Body == nil {
				modifiedVals[reqKeyData] = "(none)"
			} else {
				modifiedVals[reqKeyData] = "data with length " + fmt.Sprint(len(req.Body))
			}
		} else {
			noChangeVals[reqKeyData] = "(none)"
		}
	}

	// header removals
	if attrs.removeHeaders.set {
		for _, key := range attrs.removeHeaders.v {
			modKey := reqKey{header: key, uniqueInt: nonPredefinedAttrCount}
			nonPredefinedAttrCount++

			if req.Headers == nil {
				noChangeVals[modKey] = "not exist"
			} else {
				vals := req.Headers.Values(key)

				if len(vals) < 1 {
					noChangeVals[modKey] = "not exist"
				} else {
					// delete the most recently added header with this key.
					req.Headers.Del(key)

					oldVal := vals[len(vals)-1]
					// if there's more than one value, put the other ones back to honor
					// deleting only the most recent one
					if len(vals) > 1 {
						modifiedVals[modKey] = fmt.Sprintf("no longer have value %s", oldVal)
						for _, v := range vals[:len(vals)-1] {
							req.Headers.Add(key, v)
						}
					} else {
						modifiedVals[modKey] = "no longer exist"
					}
				}
			}
			attrOrdering = append(attrOrdering, modKey)
		}
	}

	// header adds
	if attrs.headers.set {
		if req.Headers == nil {
			req.Headers = make(http.Header)
		}

		// to make reproducible, sort the header keys first
		sortedKeys := make([]string, 0, len(attrs.headers.v))
		for key := range attrs.headers.v {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)

		for _, key := range sortedKeys {
			vals := attrs.headers.v[key]
			for _, v := range vals {
				modKey := reqKey{header: key, uniqueInt: nonPredefinedAttrCount}
				nonPredefinedAttrCount++

				modifiedVals[modKey] = fmt.Sprintf("have new value %s", v)
				req.Headers.Add(key, v)
				attrOrdering = append(attrOrdering, modKey)
			}
		}
	}

	// method and URL modifications
	if attrs.method.set {
		if req.Method != attrs.method.v {
			req.Method = attrs.method.v
			modifiedVals[reqKeyMethod] = attrs.method.v
		} else {
			noChangeVals[reqKeyMethod] = attrs.method.v
		}
	}
	if attrs.url.set {
		if req.URL != attrs.url.v {
			req.URL = attrs.url.v
			modifiedVals[reqKeyURL] = attrs.url.v
		} else {
			noChangeVals[reqKeyURL] = attrs.url.v
		}
	}

	p.Templates[strings.ToLower(req.Name)] = req

	// save the project file
	err = p.PersistToDisk(loadAllFiles)
	if err != nil {
		return err
	}

	// io mod output
	cmdio.OutputLoudEditAttrsResult(io, modifiedVals, noChangeVals, attrOrdering)

	return nil
}

func invokeReqsNew(io cmdio.IO, projFile, reqName string, attrs reqAttrValues) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqLower := strings.ToLower(reqName)
	// check if the project already has a request with the same name
	if _, exists := p.Templates[reqLower]; exists {
		return morc.NewReqExistsError(reqLower)
	}

	// create the new request template
	req := morc.RequestTemplate{
		Name:    reqName,
		Method:  attrs.method.Or("GET"),
		URL:     attrs.url.Or("http://example.com"),
		Headers: attrs.headers.v,
		Body:    attrs.body.v,
	}

	if p.Templates == nil {
		p.Templates = make(map[string]morc.RequestTemplate)
	}
	p.Templates[reqLower] = req

	// save the project file
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Created new request %s\n", reqLower)

	return nil
}

func invokeReqsDelete(io cmdio.IO, projFile, reqName string, force bool) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqLower := strings.ToLower(reqName)
	if _, ok := p.Templates[reqLower]; !ok {
		return morc.NewReqNotFoundError(reqLower)
	}

	if !force {
		// check if this req is in any flows; cannot delete it if so
		inFlows := p.FlowsWithTemplate(reqLower)

		if len(inFlows) > 0 {
			flowS := "s"
			if len(inFlows) == 1 {
				flowS = ""
			}
			return fmt.Errorf("%s is used in flow%s %s\nUse -f to force-delete", reqLower, flowS, strings.Join(inFlows, ", "))
		}
	}

	// if we are forcing, there's no checks to make.

	delete(p.Templates, reqLower)

	// save the project file
	err = p.PersistToDisk(false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Deleted request %s\n", reqLower)

	return nil
}

func invokeReqsShow(io cmdio.IO, projFile, reqName string) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqLower := strings.ToLower(reqName)
	req, ok := p.Templates[reqLower]
	if !ok {
		return morc.NewReqNotFoundError(reqLower)
	}

	// print out the request details
	meth := req.Method
	if meth == "" {
		meth = "(no-method)"
	}
	url := req.URL
	if url == "" {
		url = "(no-url)"
	}
	io.Printf("%s %s\n\n", meth, url)

	// print out headers, if any
	if len(req.Headers) > 0 {
		io.Printf("HEADERS:\n")

		// alphabetize headers
		var sortedNames []string
		for name := range req.Headers {
			sortedNames = append(sortedNames, name)
		}
		sort.Strings(sortedNames)

		for _, name := range sortedNames {
			for _, val := range req.Headers[name] {
				io.Printf("%s: %s\n", name, val)
			}
		}
	} else {
		io.Printf("HEADERS: (none)\n")
	}
	io.Printf("\n")

	if len(req.Body) > 0 {
		io.Printf("BODY:\n")
		io.Printf("%s\n", string(req.Body))
	} else {
		io.Printf("BODY: (none)\n")
	}
	io.Printf("\n")

	if len(req.Captures) > 0 {
		io.Printf("VAR CAPTURES:\n")

		// alphabetize captures
		var sortedNames []string
		for name := range req.Captures {
			sortedNames = append(sortedNames, name)
		}
		sort.Strings(sortedNames)

		for _, capName := range sortedNames {
			cap := req.Captures[capName]
			io.Printf("%s\n", cap.String())
		}
	} else {
		io.Printf("VAR CAPTURES: (none)\n")
	}
	io.Printf("\n")

	if req.AuthFlow == "" {
		io.Printf("AUTH FLOW: (none)\n")
	} else {
		io.Printf("AUTH FLOW: %s\n", req.AuthFlow)
	}

	return nil
}

func invokeReqsList(io cmdio.IO, projFile string) error {
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	if len(p.Templates) == 0 {
		io.Println("(none)")
	} else {
		// alphabetize the templates
		var sortedNames []string
		for name := range p.Templates {
			sortedNames = append(sortedNames, name)
		}
		sort.Strings(sortedNames)

		// get the longest method name
		maxLen := 0
		for _, name := range sortedNames {
			meth := p.Templates[name].Method
			if meth == "" {
				meth = "???"
			}
			if len(meth) > maxLen {
				maxLen = len(meth)
			}
		}

		for _, name := range sortedNames {
			meth := p.Templates[name].Method
			if meth == "" {
				meth = "???"
			}
			io.Printf("%-*s %s\n", maxLen, meth, name)
		}
	}

	return nil
}

func invokeReqsGet(io cmdio.IO, projFile, reqName string, item reqKey) error {
	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for request template names
	reqLower := strings.ToLower(reqName)

	req, ok := p.Templates[reqLower]
	if !ok {
		return morc.NewReqNotFoundError(reqLower)
	}

	// print out the request details
	switch item {
	case reqKeyName:
		io.Printf("%s\n", req.Name)
	case reqKeyMethod:
		if req.Method == "" {
			io.PrintLoudf("%s\n", "(none)")
		} else {
			io.Printf("%s\n", strings.ToUpper(req.Method))
		}
	case reqKeyURL:
		if req.URL == "" {
			io.PrintLoudf("%s\n", "(none)")
		} else {
			io.Printf("%s\n", req.URL)
		}
	case reqKeyData:
		if len(req.Body) == 0 {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", string(req.Body))
		}
	case reqKeyHeaders:
		if len(req.Headers) == 0 {
			io.PrintLoudf("(none)\n")
		} else {
			// alphabetize headers
			var sortedNames []string
			for name := range req.Headers {
				sortedNames = append(sortedNames, name)
			}
			sort.Strings(sortedNames)

			for _, name := range sortedNames {
				for _, val := range req.Headers[name] {
					io.Printf("%s: %s\n", name, val)
				}
			}
		}
	case reqKeyAuthFlow:
		if req.AuthFlow == "" {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", req.AuthFlow)
		}
	case reqKeyCaptures:
		if len(req.Captures) == 0 {
			io.PrintLoudf("(none)\n")
		} else {
			// alphabetize captures
			var sortedNames []string
			for name := range req.Captures {
				sortedNames = append(sortedNames, name)
			}
			sort.Strings(sortedNames)

			for _, capName := range sortedNames {
				cap := req.Captures[capName]
				io.Printf("%s\n", cap.String())
			}
		}
	default:
		// it is a header key. read the header and print its values, one per line.
		if len(req.Headers) == 0 {
			io.PrintLoudf("(none)\n")
		} else {
			vals := req.Headers.Values(item.header)
			if len(vals) == 0 {
				io.PrintLoudf("(none)\n")
			} else {
				for _, val := range vals {
					io.Printf("%s\n", val)
				}
			}
		}
	}

	return nil
}

type reqsArgs struct {
	projFile string
	action   reqsAction
	getItem  reqKey
	force    bool
	req      string

	sets reqAttrValues
}

type reqAttrValues struct {
	name          optional[string]
	method        optional[string]
	url           optional[string]
	body          optional[[]byte]
	headers       optional[http.Header]
	removeHeaders optional[[]string]
}

func parseReqsArgs(cmd *cobra.Command, posArgs []string, args *reqsArgs) error {
	args.projFile = cliflags.ProjectFile
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseReqsActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	// do action-specific arg and flag parsing
	switch args.action {
	case reqsList:
		// nothing else to do
	case reqsShow:
		// use arg 1 as the req name
		args.req = posArgs[0]
	case reqsDelete:
		// special case of req name set from a CLI flag rather than pos arg.
		args.req = cliflags.Delete

		args.force = cliflags.BForce
	case reqsGet:
		// use arg 1 as the req name
		args.req = posArgs[0]

		// user is either doing this via flagReqsGet or flagReqsGetHeader;
		// parsing is different based on which one.
		if cliflags.Get != "" {
			args.getItem, err = parseReqAttrKey(cliflags.Get)
			if err != nil {
				return err
			}
		} else {
			args.getItem = reqKey{header: cliflags.GetHeader}
		}
	case reqsNew:
		// above action parsing already checked that invalid set opts will not
		// be present so we can just call parseReqsSetFlags and then use
		// --new argument to set the new request name.
		if err := parseReqsSetFlags(cmd, &args.sets); err != nil {
			return err
		}

		// set req name from the flag
		args.req = cliflags.New
		args.sets.name = optional[string]{set: true, v: cliflags.New}
	case reqsEdit:
		// use arg 1 as the req name
		args.req = posArgs[0]

		if err := parseReqsSetFlags(cmd, &args.sets); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unhandled reqs action %q", args.action))
	}

	return nil
}

func parseReqsActionFromFlags(cmd *cobra.Command, posArgs []string) (reqsAction, error) {
	// mutual exclusions enforced by cobra (and therefore we do not check them here):
	// * --new, --delete, --get, and --get-header.
	// * --new with non-new mod flags
	// * --delete with mod flags
	// * --get with mod flags
	// * --get-header with mod flags
	// * --remove-body with --data
	// * --force with --get, --get-header, and --new

	// make sure user isn't invalidly using -f because cobra is not enforcing this
	if cliflags.BForce && cliflags.Delete == "" {
		return reqsEdit, fmt.Errorf("--force/-f can only be used with --delete/-D")
	}

	if cliflags.Delete != "" {
		if len(posArgs) > 0 {
			return reqsAction(0), fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return reqsDelete, nil
	} else if cliflags.New != "" {
		if len(posArgs) > 0 {
			return reqsAction(0), fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return reqsNew, nil
	} else if cliflags.Get != "" || cliflags.GetHeader != "" {
		if len(posArgs) < 1 {
			return reqsGet, fmt.Errorf("missing name of REQ to get from")
		}
		if len(posArgs) > 1 {
			return reqsGet, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}
		return reqsGet, nil
	} else if reqsSetFlagIsPresent(cmd) {
		if len(posArgs) < 1 {
			return reqsEdit, fmt.Errorf("missing name of REQ to update")
		}
		if len(posArgs) > 1 {
			return reqsEdit, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}
		return reqsEdit, nil
	}

	if len(posArgs) == 0 {
		return reqsList, nil
	} else if len(posArgs) == 1 {
		return reqsShow, nil
	} else {
		return reqsList, fmt.Errorf("unknown positional argument %q", posArgs[1])
	}
}

func parseReqsSetFlags(cmd *cobra.Command, attrs *reqAttrValues) error {
	f := cmd.Flags()

	if f.Changed("name") {
		attrs.name = optional[string]{set: true, v: cliflags.Name}
	}

	if f.Changed("method") {
		attrs.method = optional[string]{set: true, v: strings.ToUpper(flagReqsMethod)}
	}

	if f.Changed("url") {
		// DO NOT PARSE UNTIL SEND TIME; parsing could clobber var uses.
		attrs.url = optional[string]{set: true, v: flagReqsURL}
	}

	if f.Changed("data") {
		if strings.HasPrefix(flagReqsBodyData, "@") {
			// read entire file now
			fRaw, err := os.Open(flagReqsBodyData[1:])
			if err != nil {
				return fmt.Errorf("open %q: %w", flagReqsBodyData[1:], err)
			}
			defer fRaw.Close()
			bodyData, err := io.ReadAll(fRaw)
			if err != nil {
				return fmt.Errorf("read %q: %w", flagReqsBodyData[1:], err)
			}
			attrs.body = optional[[]byte]{set: true, v: bodyData}
		} else {
			attrs.body = optional[[]byte]{set: true, v: []byte(flagReqsBodyData)}
		}
	}

	if f.Changed("remove-body") {
		attrs.body = optional[[]byte]{set: true, v: nil}
	}

	if f.Changed("header") {
		headers := make(http.Header)
		for idx, h := range flagReqsHeaders {

			// split the header into key and value
			parts := strings.SplitN(h, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("header add #%d (%q) is not in format key: value", idx+1, h)
			}
			canonKey := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
			if canonKey == "" {
				return fmt.Errorf("header add #%d (%q) does not have a valid header key", idx+1, h)
			}
			value := strings.TrimSpace(parts[1])
			headers.Add(canonKey, value)
		}
		attrs.headers = optional[http.Header]{set: true, v: headers}
	}

	if f.Changed("remove-header") {
		delHeaders := make([]string, len(cliflags.RemoveHeaders))
		for idx, h := range cliflags.RemoveHeaders {
			trimmed := strings.TrimSpace(h)
			if strings.Contains(trimmed, " ") || strings.Contains(trimmed, ":") {
				return fmt.Errorf("header delete #%d (%q) is not a valid header key", idx+1, h)
			}
			delHeaders[idx] = trimmed
		}

		attrs.removeHeaders = optional[[]string]{set: true, v: delHeaders}
	}

	return nil
}

func reqsSetFlagIsPresent(cmd *cobra.Command) bool {
	f := cmd.Flags()
	return f.Changed("method") ||
		f.Changed("url") ||
		f.Changed("header") ||
		f.Changed("name") ||
		f.Changed("header") ||
		f.Changed("data") ||
		f.Changed("remove-header") ||
		f.Changed("remove-body")
}

type reqsAction int

const (
	reqsList reqsAction = iota
	reqsShow
	reqsNew
	reqsDelete
	reqsGet
	reqsEdit
)

type reqKey struct {
	name      string
	header    string
	uniqueInt int // only used for sorting in edit output
}

var (
	reqKeyName     reqKey = reqKey{name: "NAME"}
	reqKeyMethod   reqKey = reqKey{name: "METHOD"}
	reqKeyURL      reqKey = reqKey{name: "URL"}
	reqKeyData     reqKey = reqKey{name: "DATA"}
	reqKeyHeaders  reqKey = reqKey{name: "HEADERS"}
	reqKeyAuthFlow reqKey = reqKey{name: "AUTH"}
	reqKeyCaptures reqKey = reqKey{name: "CAPTURES"}

	// OR a specific header key denoted via leading ":".
)

// Human prints the human-readable description of the key.
func (rk reqKey) Human() string {
	if rk.header != "" {
		return fmt.Sprintf("header %s", rk.header)
	}

	switch rk.name {
	case reqKeyName.name:
		return "request name"
	case reqKeyMethod.name:
		return "request method"
	case reqKeyURL.name:
		return "request URL"
	case reqKeyData.name:
		return "request body"
	case reqKeyHeaders.name:
		return "request headers"
	case reqKeyAuthFlow.name:
		return "request auth flow"
	case reqKeyCaptures.name:
		return "request var captures"
	default:
		return fmt.Sprintf("unknown req key %q", rk.name)
	}
}

func (rk reqKey) Name() string {
	if rk.header != "" {
		return rk.header
	} else {
		return string(rk.name)
	}
}

var (
	// ordering of reqAttrKeys in output is set here

	reqAttrKeys = []reqKey{
		reqKeyName,
		reqKeyMethod,
		reqKeyURL,
		reqKeyData,
		reqKeyHeaders,
		reqKeyAuthFlow,
		reqKeyCaptures,
	}
)

func reqAttrKeyNames() []string {
	names := make([]string, len(reqAttrKeys))
	for i, k := range reqAttrKeys {
		names[i] = k.Name()
	}
	return names
}

func parseReqAttrKey(s string) (reqKey, error) {
	switch strings.ToUpper(s) {
	case reqKeyName.Name():
		return reqKeyName, nil
	case reqKeyMethod.Name():
		return reqKeyMethod, nil
	case reqKeyURL.Name():
		return reqKeyURL, nil
	case reqKeyData.Name():
		return reqKeyData, nil
	case reqKeyHeaders.Name():
		return reqKeyHeaders, nil
	case reqKeyAuthFlow.Name():
		return reqKeyAuthFlow, nil
	case reqKeyCaptures.Name():
		return reqKeyCaptures, nil
	default:
		return reqKey{}, fmt.Errorf("must be one of: %s", strings.Join(reqAttrKeyNames(), ", "))
	}
}
