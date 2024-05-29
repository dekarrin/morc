package reqs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagReqsNew           string
	flagReqsDelete        string
	flagReqsGet           string
	flagReqsGetHeader     string
	flagReqsRemoveHeaders []string
	flagReqsRemoveBody    bool
	flagReqsBodyData      string
	flagReqsHeaders       []string
	flagReqsMethod        string
	flagReqsURL           string
	flagReqsName          string
	flagReqsDeleteForce   bool
)

// TODO: the attr/index system is ridiculously overcomplicated; just use optionally-valued args ffs.
// oh no, we'll have to enforce whether a value was set. Or at least do --get to make it an explicit
// action. Then we can do normals for setting.
//
// interface would be
// REQS                                                                    LIST
// REQS --new/-N (TAKES NAME NOW) followed by other args, options.         NEW
// REQS REQ --delete/-D                                                    DELETE
// REQS REQ --get ATTR                                                     GET
// REQS REQ (implied show)                                                 SHOW
// REQS REQ (!--new) followed by flag arg sets (or extended for hdr edit)  EDIT

// -G sounds good for now.
//
// cond - if changed but value was not provided.
var ReqsCmd = &cobra.Command{
	Use: "reqs [-F FILE]\n" +
		"reqs [-F FILE] --delete REQ [-f]\n" +
		"reqs [-F FILE] --new REQ [-H HDR]... [-d DATA | -d @FILE] [-X METHOD] [-u URL] [-F FILE]\n" +
		"reqs [-F FILE] REQ\n" +
		"reqs [-F FILE] REQ --get ATTR\n" +
		"reqs [-F FILE] REQ [-ndXuHr]... [--remove-body]",
	GroupID: "project",
	Short:   "Show or modify request templates",
	Long: "Manipulate project request templates. By itself, prints out a listing of the names and methods of the request templates " +
		"in the project.\n\nA new request template can be created by providing the name of it to the --new flag and using flags to " +
		"specify attributes to set on the new request. The method of the request is set with the --method/-X flag. The payload in the " +
		"request body is set with the -d/--data flag, either directly by providing the body as the argument or indirectly by loading from " +
		"a filename given after a leading '@'. Headers are set with the -H/--header flag. Multiple headers may be specified by providing " +
		"multiple -H flags. The URL of the request is set with the the -u/--url flag.\n\nA particular request can be viewed by providing " +
		"the name of the request, REQ, as a positional argument to the flows command. This will show all details of a request template. " +
		"To see only a specific attribute of a request, provide --get along with the name of the attribute of the request to show. " +
		"The attribute, ATTR, must be one of the following: " +
		strings.Join(reqAttrKeyNames(), ", ") + ". If 'HEADERS' is selected, all headers on the request are printed. To see the value(s) of " +
		"only a particular header, use --get-header with the name of the header to see instead.\n\n" +
		"Modifications to existing request templates are performed by giving REQ as a positional argument followed by one or more flag " +
		"that sets a property of the request. For example, to change the method of a request, provide the -X flag followed by the new " +
		"method. All flags that are supported during request creation are also supported when modifying a request (-X, -d, -u, -H), in " +
		"addition to a few others. The name of the request is updated with -n/--name. Since -H only *adds* new header values, " +
		"--remove-header/-r can be used to remove all values of an existing header from the request. Removing only a single value of " +
		"a multi-valued header is not supported at this time; it is all or nothing. Finally, calling --remove-body will remove the " +
		"body payload entirely, which may differ from simply setting it to the empty string.\n\n" +
		"Requests are deleted by passing the --delete flag with a request name as its argument. This will irreversibly remove the request " +
		"from the project entirely.",
	Args: cobra.NoArgs,
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
	ReqsCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsNew, "new", "N", "", "Create a new request template named `REQ`.")
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsDelete, "delete", "D", "", "Delete the request template named `REQ`.")
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsGet, "get", "G", "", "Get the value of the given attribute `ATTR` from the request. To get a particular header's value, use --get-header instead. ATTR must be one of: "+strings.Join(reqAttrKeyNames(), ", "))
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsGetHeader, "get-header", "", "", "Get the value(s) of the given header `KEY` that is currently set on the request.")
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsName, "name", "n", "", "Change the name of a request template to `NAME`.")
	ReqsCmd.PersistentFlags().StringArrayVarP(&flagReqsRemoveHeaders, "remove-header", "r", []string{}, "Remove header with key `KEY` from the request. If multiple headers with the same key exist, only the most recently added one will be deleted.")
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsBodyData, "data", "d", "", "Add the given `DATA` as a body to the request; prefix with @ to read data from a file")
	ReqsCmd.PersistentFlags().StringArrayVarP(&flagReqsHeaders, "header", "H", []string{}, "Add a header to the request. Format is `KEY:VALUE`. Multiple headers may be set by providing multiple -H flags. If multiple headers with the same key are set, they will be set in the order they were given.")
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsMethod, "method", "X", "GET", "Set the request method to `METHOD`.")
	ReqsCmd.PersistentFlags().StringVarP(&flagReqsURL, "url", "u", "http://example.com", "Specify the `URL` for the request.")
	ReqsCmd.PersistentFlags().BoolVarP(&flagReqsRemoveBody, "remove-body", "", false, "Delete all existing body data from the request")
	ReqsCmd.PersistentFlags().BoolVarP(&flagReqsDeleteForce, "force", "f", false, "Force deletion of the request template even if it is used in flows. Only valid with --delete/-D.")

	ReqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "name")
	ReqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "remove-header")
	ReqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "remove-body")
	ReqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "data")
	ReqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "method")
	ReqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "header")
	ReqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "url")
	ReqsCmd.MarkFlagsMutuallyExclusive("data", "remove-body")
	ReqsCmd.MarkFlagsMutuallyExclusive("new", "get", "get-header", "force")
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
			return fmt.Errorf("%s is used in flow%s %s\nUse -f to force the deletion", reqLower, flowS, strings.Join(inFlows, ", "))
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
		for _, cap := range req.Captures {
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

			for name, vals := range sortedNames {
				for _, val := range vals {
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
			for _, cap := range req.Captures {
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
	args.projFile = commonflags.ProjectFile
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
		args.req = flagReqsDelete

		args.force = flagReqsDeleteForce
	case reqsGet:
		// use arg 1 as the req name
		args.req = posArgs[0]

		// user is either doing this via flagReqsGet or flagReqsGetHeader;
		// parsing is different based on which one.
		if flagReqsGet != "" {
			args.getItem, err = parseReqAttrKey(flagReqsGet)
			if err != nil {
				return err
			}
		} else {
			args.getItem = reqKey{header: flagReqsGetHeader}
		}
	case reqsNew:
		// above action parsing already checked that invalid set opts will not
		// be present so we can just call parseReqsSetFlags and then use
		// --new argument to set the new request name.
		if err := parseReqsSetFlags(cmd, &args.sets); err != nil {
			return err
		}

		// set req name from the flag
		args.req = flagReqsNew
		args.sets.name = optional[string]{set: true, v: flagReqsNew}
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
	if flagReqsDeleteForce && flagReqsDelete == "" {
		return reqsEdit, fmt.Errorf("--force/-f can only be used with --delete/-D")
	}

	if flagReqsDelete != "" {
		if len(posArgs) > 0 {
			return reqsAction(0), fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return reqsDelete, nil
	} else if flagReqsNew != "" {
		if len(posArgs) > 0 {
			return reqsAction(0), fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return reqsNew, nil
	} else if flagReqsGet != "" || flagReqsGetHeader != "" {
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
		attrs.name = optional[string]{set: true, v: flagReqsName}
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
			fRaw, err := os.Open(flagEditBodyData[1:])
			if err != nil {
				return fmt.Errorf("open %q: %w", flagEditBodyData[1:], err)
			}
			defer fRaw.Close()
			bodyData, err := io.ReadAll(fRaw)
			if err != nil {
				return fmt.Errorf("read %q: %w", flagEditBodyData[1:], err)
			}
			attrs.body = optional[[]byte]{set: true, v: bodyData}
		} else {
			attrs.body = optional[[]byte]{set: true, v: []byte(flagEditBodyData)}
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
		delHeaders := make([]string, len(flagReqsRemoveHeaders))
		for idx, h := range flagReqsRemoveHeaders {
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
	name   string
	header string
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
		return "request body data"
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
		return fmt.Sprintf(":%s", rk.header)
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
