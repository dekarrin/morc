package reqs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagReqsNew    bool
	flagReqsDelete bool
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
		"reqs [-F FILE] --delete REQ\n" +
		"reqs [-F FILE] --new REQ [-H HDR]... [-d DATA | -d @FILE] [-X METHOD] [-u URL] [-F FILE]\n" +
		"reqs [-F FILE] REQ\n" +
		"reqs [-F FILE] REQ --get ATTR\n" +
		"reqs [-F FILE] REQ [-ndXuH]...",
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
		"The attribute, ATTR, can either be a header key with a leading ':' or one of the following: " +
		strings.Join(reqAttrKeyNames(), ", ") + ". If 'HEADERS' is selected, all headers on the request are printed. To see the value(s) of " +
		"only a particular header, provide the name of the header as the argument to --get, prepended with a leading ':'.\n\n" +
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
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := commonflags.ProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// ACTION PARSE SKETCHING:

		// reqs - LIST
		// reqs REQ > SHOW
		// reqs REQ -d > DELETE (cannot be used with --new ever)
		// reqs REQ --new (other set flags)/ATTR1 VALUE1 > NEW
		// reqs REQ (other set flags)/ATTR1 VALUE1 > EDIT
		// reqs REQ ATTR > GET

		// reqs REQ HDR - LIST HDRS
		// reqs REQ -H - LIST HDRS
		// reqs REQ --header-value 'my-key' - GET HDR
		// reqs REQ -H 'my-key: value' - SET HDR
		// reqs REQ -v/--header-value 'my-key'
		// reqs REQ

		// First, sanity arg checking.
		//
		// * mut excl --new and -d
		// * mut excl -d and all other set options
		// * if --delete set, Nargs = 1.
		// * if --new set, Nargs >= 1 and Nargs-1 %2 == 0.
		// * if set option set, check -d must not be set. Nargs must be >=1 and Nargs-1 %2 == 0.
		//
		// args should be valid in all cases now, sans certain ones.
		//
		// if Nargs == 0:
		// * set options not allowed (ALREADY CHECKED)
		// * --new not allowed (ALREADY CHECKED)
		// * --delete not allowed (ALREADY CHECKED)
		// * MODE = LIST
		// if Nargs == 1:
		// * if --delete set:
		//   * set options not allowed (ALREADY CHECKED)
		//   * --new not allowed (ALREADY CHECKED)
		//   * MODE = DELETE
		// * else if --new set:
		//   * --delete not allowed (ALREADY CHECKED)
		//   * MODE = NEW
		//   * PROCESS ATTR ARGS
		// * else if any set option set:
		//   * MODE = EDIT
		// * else:
		//   * MODE = SHOW
		// if Nargs >= 2:
		//   // either a GET, EDIT, or NEW
		//   // * process attr

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		return invokeReqList(io, filename)
	},
}

func init() {
	ReqsCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
}

func invokeReqList(io cmdio.IO, filename string) error {
	p, err := morc.LoadProjectFromDisk(filename, true)
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
	reqKeyName    reqKey = reqKey{name: "NAME"}
	reqKeyMethod  reqKey = reqKey{name: "METHOD"}
	reqKeyURL     reqKey = reqKey{name: "URL"}
	reqKeyData    reqKey = reqKey{name: "DATA"}
	reqKeyHeaders reqKey = reqKey{name: "HEADERS"}

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
	default:
		// must be at least 2 chars or it may as well be empty.
		if len(s) > 1 && s[0] == ':' {
			return reqKey{header: s[1:]}, nil
		} else {
			return reqKey{}, fmt.Errorf("must be a header key starting with ':' or one of: %s", strings.Join(reqAttrKeyNames(), ", "))
		}
	}
}
