package commands

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

// INVOKES WE MUST SUPPORT:
// cli invoke: morc auths -N basic-login -t basic -u username -p password
// cli invoke: morc auths -N auth-name -t session -r retrievial-spec -c cookie --no-exp
// cli invoke: morc auths -N auth-name -t token -r retrieval-spec --token-from token-scraper --use-in header:key-name --use-format bearer --exp expires-scraper --exp-layout expires-layout
// cli invoke: morc auths -N auth-name -t jwt -r retrieval-spec --token-from token-scraper

var authsCmd = &cobra.Command{
	Use: "auths [AUTH]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"auths\n" +
			"auths --delete AUTH [-f]\n" +
			"auths --new AUTH \n" + // TODO: actual auth parameters
			"auths AUTH\n" +
			"auths AUTH --get ATTR\n" +
			"auths AUTH \n" + // TODO: actual auth parameters.
			"auths AUTH --clear",
	},
	GroupID: "project",
	Short:   "Show or modify authorization methods",
	Long:    "", // TODO: fill help.
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args reqsArgs
		if err := parseReqsArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		io.Quiet = flags.BQuiet

		switch args.action {
		case reqsActionList:
			return invokeReqsList(io, args.projFile)
		case reqsActionShow:
			return invokeReqsShow(io, args.projFile, args.req)
		case reqsActionDelete:
			return invokeReqsDelete(io, args.projFile, args.req, args.force)
		case reqsActionGet:
			return invokeReqsGet(io, args.projFile, args.req, args.getItem)
		case reqsActionNew:
			return invokeReqsNew(io, args.projFile, args.req, args.sets)
		case reqsActionEdit:
			return invokeReqsEdit(io, args.projFile, args.req, args.sets)
		default:
			panic(fmt.Sprintf("unhandled reqs action %q", args.action))
		}
	},
}

func init() {

	// cli invoke: morc auths -N basic-login -t basic -u username -p password
	// cli invoke: morc auths -N auth-name -t session -r retrievial-spec -c cookie --no-exp
	// cli invoke: morc auths -N auth-name -t token -r retrieval-spec --token-from token-scraper --use-in header:key-name --use-format bearer --exp expires-scraper --exp-layout expires-layout
	// cli invoke: morc auths -N auth-name -t jwt -r retrieval-spec --token-from token-scraper
	authsCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	authsCmd.PersistentFlags().StringVarP(&flags.New, "new", "N", "", "Create a new auth method named `AUTH`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Delete, "delete", "D", "", "Delete the auth method named `AUTH`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Get, "get", "G", "", "Get the value of the given attribute `ATTR` from the auth method. ATTR must be one of: "+strings.Join(authAttrKeyNames(), ", "))
	authsCmd.PersistentFlags().BoolVarP(&flags.BClear, "clear", "C", false, "Clear any currently saved auth proof. Only applicable to auth types that use dynamically-retrieved proofs, such a cookie or a token.")
	authsCmd.PersistentFlags().StringVarP(&flags.Name, "name", "n", "", "Change the name of an auth method to `NAME`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Type, "type", "t", "", "Set the type of auth method to `TYPE`. TYPE must be one of 'basic', 'session', 'jwt', or 'token'; the choice determined what other options are available.")
	authsCmd.PersistentFlags().StringVarP(&flags.Username, "username", "u", "", "Set the `USERNAME` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Password, "password", "p", "", "Set the `PASSWORD` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Cookie, "cookie", "c", "", "Set the name of the cookie to get session information from to `NAME`. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BNoExpiration, "no-exp", "", false, "Do not detect an expiration time for the session cookie, resulting in it being used until an invalid auth is detected. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Retrieval, "retrieval", "r", "", "Set the flow or request template to use to retrieve proof of authentication. This is a string of the form F:NAME for a flow or R:NAME for a request template; if no prefix is given, it is assumed to be a flow name. Only valid when --type is 'session', 'jwt', or 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.TokenScraper, "token-from", "", "", "Set the scraper to use to extract the token from the response. Only valid when --type is 'jwt' or 'token'.")

	// reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "name")
	// reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "remove-header")
	// reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "get-header", "remove-body")
	// reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "data")
	// reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "method")
	// reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "header")
	// reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "get-header", "url")
	// reqsCmd.MarkFlagsMutuallyExclusive("data", "remove-body")
	// reqsCmd.MarkFlagsMutuallyExclusive("new", "get", "get-header", "force")

	rootCmd.AddCommand(authsCmd)
}

// func invokeReqsEdit(io cmdio.IO, projFile, reqName string, attrs reqAttrValues) error {
// 	// first, are we updating the name? this determines whether we need to load
// 	// and save history
// 	loadAllFiles := attrs.name.set

// 	// load the project file
// 	p, err := readProject(projFile, loadAllFiles)
// 	if err != nil {
// 		return err
// 	}

// 	// case doesn't matter for request template names
// 	reqLower := strings.ToLower(reqName)
// 	req, ok := p.Templates[reqLower]
// 	if !ok {
// 		return morc.NewReqNotFoundError(reqLower)
// 	}

// 	modifiedVals := map[reqKey]interface{}{}
// 	noChangeVals := map[reqKey]interface{}{}

// 	// build up order slice as we go to contain our non-predefined values
// 	attrOrdering := make([]reqKey, len(reqAttrKeys))
// 	copy(attrOrdering, reqAttrKeys)
// 	nonPredefinedAttrCount := 0

// 	// if changing names, do that first
// 	if attrs.name.set {
// 		newName := strings.ToLower(attrs.name.v)

// 		if newName != reqLower {
// 			if _, exists := p.Templates[newName]; exists {
// 				return morc.NewReqExistsError(newName)
// 			}

// 			// update the name in the history
// 			for idx, h := range p.History {
// 				if strings.ToLower(h.Template) == reqLower {
// 					p.History[idx].Template = newName
// 				}
// 			}

// 			// update the name in the flows
// 			for flowName, flow := range p.Flows {
// 				for idx, step := range flow.Steps {
// 					if strings.ToLower(step.Template) == reqLower {
// 						p.Flows[flowName].Steps[idx].Template = newName
// 					}
// 				}
// 			}

// 			// update the name in the project
// 			req.Name = newName
// 			delete(p.Templates, reqLower)
// 			modifiedVals[reqKeyName] = newName
// 		} else {
// 			noChangeVals[reqKeyName] = newName
// 		}
// 	}

// 	// any name changes will have gone to req.Name at this point; be shore to
// 	// use that for any name displaying from this point forward

// 	// body modifications
// 	if attrs.body.set {
// 		if !(attrs.body.v == nil && req.Body == nil) {
// 			req.Body = attrs.body.v

// 			if req.Body == nil {
// 				modifiedVals[reqKeyData] = "(none)"
// 			} else {
// 				modifiedVals[reqKeyData] = "data with length " + fmt.Sprint(len(req.Body))
// 			}
// 		} else {
// 			noChangeVals[reqKeyData] = "(none)"
// 		}
// 	}

// 	// header removals
// 	if attrs.removeHeaders.set {
// 		for _, key := range attrs.removeHeaders.v {
// 			modKey := reqKey{header: key, uniqueInt: nonPredefinedAttrCount}
// 			nonPredefinedAttrCount++

// 			if req.Headers == nil {
// 				noChangeVals[modKey] = "not exist"
// 			} else {
// 				vals := req.Headers.Values(key)

// 				if len(vals) < 1 {
// 					noChangeVals[modKey] = "not exist"
// 				} else {
// 					// delete the most recently added header with this key.
// 					req.Headers.Del(key)

// 					oldVal := vals[len(vals)-1]
// 					// if there's more than one value, put the other ones back to honor
// 					// deleting only the most recent one
// 					if len(vals) > 1 {
// 						modifiedVals[modKey] = fmt.Sprintf("no longer have value %s", oldVal)
// 						for _, v := range vals[:len(vals)-1] {
// 							req.Headers.Add(key, v)
// 						}
// 					} else {
// 						modifiedVals[modKey] = "no longer exist"
// 					}
// 				}
// 			}
// 			attrOrdering = append(attrOrdering, modKey)
// 		}
// 	}

// 	// header adds
// 	if attrs.headers.set {
// 		if req.Headers == nil {
// 			req.Headers = make(http.Header)
// 		}

// 		// to make reproducible, sort the header keys first
// 		sortedKeys := make([]string, 0, len(attrs.headers.v))
// 		for key := range attrs.headers.v {
// 			sortedKeys = append(sortedKeys, key)
// 		}
// 		sort.Strings(sortedKeys)

// 		for _, key := range sortedKeys {
// 			vals := attrs.headers.v[key]
// 			for _, v := range vals {
// 				modKey := reqKey{header: key, uniqueInt: nonPredefinedAttrCount}
// 				nonPredefinedAttrCount++

// 				modifiedVals[modKey] = fmt.Sprintf("have new value %s", v)
// 				req.Headers.Add(key, v)
// 				attrOrdering = append(attrOrdering, modKey)
// 			}
// 		}
// 	}

// 	// method and URL modifications
// 	if attrs.method.set {
// 		if req.Method != attrs.method.v {
// 			req.Method = attrs.method.v
// 			modifiedVals[reqKeyMethod] = attrs.method.v
// 		} else {
// 			noChangeVals[reqKeyMethod] = attrs.method.v
// 		}
// 	}
// 	if attrs.url.set {
// 		if req.URL != attrs.url.v {
// 			req.URL = attrs.url.v
// 			modifiedVals[reqKeyURL] = attrs.url.v
// 		} else {
// 			noChangeVals[reqKeyURL] = attrs.url.v
// 		}
// 	}

// 	p.Templates[strings.ToLower(req.Name)] = req

// 	// save the project file
// 	err = writeProject(p, loadAllFiles)
// 	if err != nil {
// 		return err
// 	}

// 	// io mod output
// 	cmdio.OutputLoudEditAttrsResult(io, modifiedVals, noChangeVals, attrOrdering)

// 	return nil
// }

// func invokeReqsNew(io cmdio.IO, projFile, reqName string, attrs reqAttrValues) error {
// 	// load the project file
// 	p, err := readProject(projFile, true)
// 	if err != nil {
// 		return err
// 	}

// 	// case doesn't matter for request template names
// 	reqLower := strings.ToLower(reqName)
// 	// check if the project already has a request with the same name
// 	if _, exists := p.Templates[reqLower]; exists {
// 		return morc.NewReqExistsError(reqLower)
// 	}

// 	// create the new request template
// 	req := morc.RequestTemplate{
// 		Name:    reqName,
// 		Method:  attrs.method.Or("GET"),
// 		URL:     attrs.url.Or("http://example.com"),
// 		Headers: attrs.headers.v,
// 		Body:    attrs.body.v,
// 	}

// 	if p.Templates == nil {
// 		p.Templates = make(map[string]morc.RequestTemplate)
// 	}
// 	p.Templates[reqLower] = req

// 	// save the project file
// 	err = writeProject(p, false)
// 	if err != nil {
// 		return err
// 	}

// 	io.PrintLoudf("Created new request %s\n", reqLower)

// 	return nil
// }

// func invokeReqsDelete(io cmdio.IO, projFile, reqName string, force bool) error {
// 	// load the project file
// 	p, err := readProject(projFile, true)
// 	if err != nil {
// 		return err
// 	}

// 	// case doesn't matter for request template names
// 	reqLower := strings.ToLower(reqName)
// 	if _, ok := p.Templates[reqLower]; !ok {
// 		return morc.NewReqNotFoundError(reqLower)
// 	}

// 	if !force {
// 		// check if this req is in any flows; cannot delete it if so
// 		inFlows := p.FlowsWithTemplate(reqLower)

// 		if len(inFlows) > 0 {
// 			flowS := "s"
// 			if len(inFlows) == 1 {
// 				flowS = ""
// 			}
// 			return fmt.Errorf("%s is used in flow%s %s\nUse -f to force-delete", reqLower, flowS, strings.Join(inFlows, ", "))
// 		}
// 	}

// 	// if we are forcing, there's no checks to make.

// 	delete(p.Templates, reqLower)

// 	// save the project file
// 	err = writeProject(p, false)
// 	if err != nil {
// 		return err
// 	}

// 	io.PrintLoudf("Deleted request %s\n", reqLower)

// 	return nil
// }

// func invokeReqsShow(io cmdio.IO, projFile, reqName string) error {
// 	// load the project file
// 	p, err := readProject(projFile, true)
// 	if err != nil {
// 		return err
// 	}

// 	// case doesn't matter for request template names
// 	reqLower := strings.ToLower(reqName)
// 	req, ok := p.Templates[reqLower]
// 	if !ok {
// 		return morc.NewReqNotFoundError(reqLower)
// 	}

// 	// print out the request details
// 	meth := req.Method
// 	if meth == "" {
// 		meth = "(no-method)"
// 	}
// 	url := req.URL
// 	if url == "" {
// 		url = "(no-url)"
// 	}
// 	io.Printf("%s %s\n\n", meth, url)

// 	// print out headers, if any
// 	if len(req.Headers) > 0 {
// 		io.Printf("HEADERS:\n")

// 		// alphabetize headers
// 		var sortedNames []string
// 		for name := range req.Headers {
// 			sortedNames = append(sortedNames, name)
// 		}
// 		sort.Strings(sortedNames)

// 		for _, name := range sortedNames {
// 			for _, val := range req.Headers[name] {
// 				io.Printf("%s: %s\n", name, val)
// 			}
// 		}
// 	} else {
// 		io.Printf("HEADERS:")
// 		io.PrintLoudf(" (none)")
// 		io.Printf("\n")
// 	}
// 	io.Printf("\n")

// 	if len(req.Body) > 0 {
// 		io.Printf("BODY:\n")
// 		io.Printf("%s\n", string(req.Body))
// 	} else {
// 		io.Printf("BODY:")
// 		io.PrintLoudf(" (none)")
// 		io.Printf("\n")
// 	}
// 	io.Printf("\n")

// 	if len(req.Captures) > 0 {
// 		io.Printf("VAR CAPTURES:\n")

// 		// alphabetize captures
// 		var sortedNames []string
// 		for name := range req.Captures {
// 			sortedNames = append(sortedNames, name)
// 		}
// 		sort.Strings(sortedNames)

// 		for _, capName := range sortedNames {
// 			cap := req.Captures[capName]
// 			io.Printf("%s%s\n", p.VarPrefix(), cap.String())
// 		}
// 	} else {
// 		io.Printf("VAR CAPTURES:")
// 		io.PrintLoudf(" (none)")
// 		io.Printf("\n")
// 	}
// 	io.Printf("\n")

// 	if req.Auth == "" {
// 		io.Printf("AUTH FLOW:")
// 		io.PrintLoudf(" (none)")
// 		io.Printf("\n")
// 	} else {
// 		io.Printf("AUTH FLOW: %s\n", req.Auth)
// 	}

// 	return nil
// }

// func invokeReqsList(io cmdio.IO, projFile string) error {
// 	p, err := readProject(projFile, true)
// 	if err != nil {
// 		return err
// 	}

// 	if len(p.Templates) == 0 {
// 		io.PrintLoudln("(none)")
// 	} else {
// 		// alphabetize the templates
// 		var sortedNames []string
// 		for name := range p.Templates {
// 			sortedNames = append(sortedNames, name)
// 		}
// 		sort.Strings(sortedNames)

// 		// get the longest method name
// 		maxLen := 0
// 		for _, name := range sortedNames {
// 			meth := p.Templates[name].Method
// 			if meth == "" {
// 				meth = "???"
// 			}
// 			if len(meth) > maxLen {
// 				maxLen = len(meth)
// 			}
// 		}

// 		for _, name := range sortedNames {
// 			meth := p.Templates[name].Method
// 			if meth == "" {
// 				meth = "???"
// 			}
// 			io.Printf("%-*s %s\n", maxLen, meth, name)
// 		}
// 	}

// 	return nil
// }

// func invokeReqsGet(io cmdio.IO, projFile, reqName string, item reqKey) error {
// 	// load the project file
// 	p, err := readProject(projFile, true)
// 	if err != nil {
// 		return err
// 	}

// 	// case doesn't matter for request template names
// 	reqLower := strings.ToLower(reqName)

// 	req, ok := p.Templates[reqLower]
// 	if !ok {
// 		return morc.NewReqNotFoundError(reqLower)
// 	}

// 	// print out the request details
// 	switch item {
// 	case reqKeyName:
// 		io.Printf("%s\n", req.Name)
// 	case reqKeyMethod:
// 		if req.Method == "" {
// 			io.PrintLoudf("%s\n", "(none)")
// 		} else {
// 			io.Printf("%s\n", strings.ToUpper(req.Method))
// 		}
// 	case reqKeyURL:
// 		if req.URL == "" {
// 			io.PrintLoudf("%s\n", "(none)")
// 		} else {
// 			io.Printf("%s\n", req.URL)
// 		}
// 	case reqKeyData:
// 		if len(req.Body) == 0 {
// 			io.PrintLoudf("(none)\n")
// 		} else {
// 			io.Printf("%s\n", string(req.Body))
// 		}
// 	case reqKeyHeaders:
// 		if len(req.Headers) == 0 {
// 			io.PrintLoudf("(none)\n")
// 		} else {
// 			// alphabetize headers
// 			var sortedNames []string
// 			for name := range req.Headers {
// 				sortedNames = append(sortedNames, name)
// 			}
// 			sort.Strings(sortedNames)

// 			for _, name := range sortedNames {
// 				for _, val := range req.Headers[name] {
// 					io.Printf("%s: %s\n", name, val)
// 				}
// 			}
// 		}
// 	case reqKeyAuthFlow:
// 		if req.Auth == "" {
// 			io.PrintLoudf("(none)\n")
// 		} else {
// 			io.Printf("%s\n", req.Auth)
// 		}
// 	case reqKeyCaptures:
// 		if len(req.Captures) == 0 {
// 			io.PrintLoudf("(none)\n")
// 		} else {
// 			// alphabetize captures
// 			var sortedNames []string
// 			for name := range req.Captures {
// 				sortedNames = append(sortedNames, name)
// 			}
// 			sort.Strings(sortedNames)

// 			for _, capName := range sortedNames {
// 				cap := req.Captures[capName]
// 				io.Printf("%s%s\n", p.VarPrefix(), cap.String())
// 			}
// 		}
// 	default:
// 		// it is a header key. read the header and print its values, one per line.
// 		if len(req.Headers) == 0 {
// 			io.PrintLoudf("(none)\n")
// 		} else {
// 			vals := req.Headers.Values(item.header)
// 			if len(vals) == 0 {
// 				io.PrintLoudf("(none)\n")
// 			} else {
// 				for _, val := range vals {
// 					io.Printf("%s\n", val)
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }

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
	args.projFile = projPathFromFlagsOrFile(cmd)
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
	case reqsActionList:
		// nothing else to do
	case reqsActionShow:
		// use arg 1 as the req name
		args.req = posArgs[0]
	case reqsActionDelete:
		// special case of req name set from a CLI flag rather than pos arg.
		args.req = flags.Delete

		args.force = flags.BForce
	case reqsActionGet:
		// use arg 1 as the req name
		args.req = posArgs[0]

		// user is either doing this via --get or --get-header;
		// parsing is different based on which one.
		if flags.Get != "" {
			args.getItem, err = parseReqAttrKey(flags.Get)
			if err != nil {
				return err
			}
		} else {
			args.getItem = reqKey{header: flags.GetHeader}
		}
	case reqsActionNew:
		// above action parsing already checked that invalid set opts will not
		// be present so we can just call parseReqsSetFlags and then use
		// --new argument to set the new request name.
		if err := parseReqsSetFlags(cmd, &args.sets); err != nil {
			return err
		}

		// set req name from the flag
		args.req = flags.New
		args.sets.name = optional[string]{set: true, v: flags.New}
	case reqsActionEdit:
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

// func parseReqsActionFromFlags(cmd *cobra.Command, posArgs []string) (reqsAction, error) {
// 	// mutual exclusions enforced by cobra (and therefore we do not check them here):
// 	// * --new, --delete, --get, and --get-header.
// 	// * --new with non-new mod flags
// 	// * --delete with mod flags
// 	// * --get with mod flags
// 	// * --get-header with mod flags
// 	// * --remove-body with --data
// 	// * --force with --get, --get-header, and --new

// 	// make sure user isn't invalidly using -f because cobra is not enforcing this
// 	if flags.BForce && flags.Delete == "" {
// 		return reqsActionEdit, fmt.Errorf("--force/-f can only be used with --delete/-D")
// 	}

// 	if flags.Delete != "" {
// 		if len(posArgs) > 0 {
// 			return reqsAction(0), fmt.Errorf("unknown positional argument %q", posArgs[0])
// 		}
// 		return reqsActionDelete, nil
// 	} else if flags.New != "" {
// 		if len(posArgs) > 0 {
// 			return reqsAction(0), fmt.Errorf("unknown positional argument %q", posArgs[0])
// 		}
// 		return reqsActionNew, nil
// 	} else if flags.Get != "" || flags.GetHeader != "" {
// 		if len(posArgs) < 1 {
// 			return reqsActionGet, fmt.Errorf("missing name of REQ to get from")
// 		}
// 		if len(posArgs) > 1 {
// 			return reqsActionGet, fmt.Errorf("unknown positional argument %q", posArgs[1])
// 		}
// 		return reqsActionGet, nil
// 	} else if reqsSetFlagIsPresent(cmd) {
// 		if len(posArgs) < 1 {
// 			return reqsActionEdit, fmt.Errorf("missing name of REQ to update")
// 		}
// 		if len(posArgs) > 1 {
// 			return reqsActionEdit, fmt.Errorf("unknown positional argument %q", posArgs[1])
// 		}
// 		return reqsActionEdit, nil
// 	}

// 	if len(posArgs) == 0 {
// 		return reqsActionList, nil
// 	} else if len(posArgs) == 1 {
// 		return reqsActionShow, nil
// 	} else {
// 		return reqsActionList, fmt.Errorf("unknown positional argument %q", posArgs[1])
// 	}
// }

// func parseReqsSetFlags(cmd *cobra.Command, attrs *reqAttrValues) error {
// 	f := cmd.Flags()

// 	if f.Changed("name") {
// 		attrs.name = optional[string]{set: true, v: flags.Name}
// 	}

// 	if f.Changed("method") {
// 		attrs.method = optional[string]{set: true, v: strings.ToUpper(flags.Method)}
// 	}

// 	if f.Changed("url") {
// 		// DO NOT PARSE UNTIL SEND TIME; parsing could clobber var uses.
// 		attrs.url = optional[string]{set: true, v: flags.URL}
// 	}

// 	if f.Changed("data") {
// 		if strings.HasPrefix(flags.BodyData, "@") {
// 			// read entire file now
// 			fRaw, err := os.Open(flags.BodyData[1:])
// 			if err != nil {
// 				return fmt.Errorf("open %q: %w", flags.BodyData[1:], err)
// 			}
// 			defer fRaw.Close()
// 			bodyData, err := io.ReadAll(fRaw)
// 			if err != nil {
// 				return fmt.Errorf("read %q: %w", flags.BodyData[1:], err)
// 			}
// 			attrs.body = optional[[]byte]{set: true, v: bodyData}
// 		} else {
// 			attrs.body = optional[[]byte]{set: true, v: []byte(flags.BodyData)}
// 		}
// 	}

// 	if f.Changed("remove-body") {
// 		attrs.body = optional[[]byte]{set: true, v: nil}
// 	}

// 	if f.Changed("header") {
// 		headers := make(http.Header)
// 		for idx, h := range flags.Headers {

// 			// split the header into key and value
// 			parts := strings.SplitN(h, ":", 2)
// 			if len(parts) != 2 {
// 				return fmt.Errorf("header add #%d (%q) is not in format key: value", idx+1, h)
// 			}
// 			canonKey := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
// 			if canonKey == "" {
// 				return fmt.Errorf("header add #%d (%q) does not have a valid header key", idx+1, h)
// 			}
// 			value := strings.TrimSpace(parts[1])
// 			headers.Add(canonKey, value)
// 		}
// 		attrs.headers = optional[http.Header]{set: true, v: headers}
// 	}

// 	if f.Changed("remove-header") {
// 		delHeaders := make([]string, len(flags.RemoveHeaders))
// 		for idx, h := range flags.RemoveHeaders {
// 			trimmed := strings.TrimSpace(h)
// 			if strings.Contains(trimmed, " ") || strings.Contains(trimmed, ":") {
// 				return fmt.Errorf("header delete #%d (%q) is not a valid header key", idx+1, h)
// 			}
// 			delHeaders[idx] = trimmed
// 		}

// 		attrs.removeHeaders = optional[[]string]{set: true, v: delHeaders}
// 	}

// 	return nil
// }

type authsAction int

const (
	authsActionList authsAction = iota
	authsActionShow
	authsActionNew
	authsActionDelete
	authsActionGet
	authsActionEdit
)
