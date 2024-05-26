package reqs

import (
	"fmt"
	"sort"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagReqsNew    bool
	flagReqsDelete bool
)

func init() {
	ReqsCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
}

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
//
// PROJ:
// PROJ --new/-N (no name required), followed by specific args.      NEW
// PROJ --get ATTR                                                   GET
// PROJ (implied show)                                               SHOW
// PROJ (!--new,!--get) followed by flag arg sets                    EDIT
//
// CAPS:
// CAPS REQ                                                          LIST
// CAPS REQ --new (REQUIRES SPECIFICATION)                           NEW
// CAPS REQ CAP --delete                                             DELETE
// CAPS REQ CAP --get ATTR 										     GET
// CAPS REQ CAP (implied show)                                       SHOW
// CAPS REQ CAP (!--new) followed by flag arg sets                   EDIT
//
// FLOWS:
// FLOWS 														        LIST
// FLOWS --new/-N (NAME REQUIRED) followed by manual check, min 2 reqs  NEW
// FLOWS FLOW --delete                                                  DELETE
// FLOWS FLOW --get ATTR  											    GET
// FLOWS FLOW (implied show) 										    SHOW
// FLOWS FLOW (!--new) followed by flag arg sets extended for step mod  EDIT

// -G sounds good for now.
//
// cond - if changed but value was not provided.
var ReqsCmd = &cobra.Command{
	Use: "reqs [-F FILE]\n" +
		"reqs REQ --new [ATTR VALUE]... [-H HDR]... [-d DATA | -d @FILE] [-X METHOD] [-u URL] [-F FILE]\n" +
		"reqs REQ [-F FILE]\n" +
		"reqs REQ --delete [-F FILE]\n" +
		"reqs REQ ATTR [-dXuH] [-v HDR-KEY] [-F FILE]\n" +
		"reqs REQ [ATTR VALUE]... [-dXuH]... [-F FILE]",
	GroupID: "project",
	Short:   "Show or modify request templates",
	Long: "Manipulate project request templates. By itself, prints out a listing of the names and methods of the request templates " +
		"in the project.\n\nA new request template can be created with the --new flag along with the name of the new request, REQ, and " +
		"any attributes to set on the new request along with their values. Some attributes may be specified by name or by flag. The " +
		"method of the request is set with either the METHOD attribute and a value or the -X/--method flag. The payload in the request " +
		"body can be set either by providing the DATA attribute and a value or the -d/--data flag. Headers are set with the -H/--header " +
		"flag; they cannot be specified with attributes. Multiple headers may be specified by providing multiple -H flags. The URL of the " +
		"request is set with the URL attribute and a value or " +
		"the -u/--url flag.\n\nA particular request is shown by providing only the name of the request, REQ, with no other options. This " +
		"will show all details of a request template. To see only a specific attribute of a request, provide the name of the request REQ " +
		"along with the name of the attribute to show. Alternatively, a single flag without a value can be provided to get only the " +
		"associated attribute: -X, -d, -u, or -H. -H behaves a bit differently; it will cause all headers to be printed. A single header's " +
		"values can be selected by instead giving -v/--header-value along with the name of a header key. This will print only the value(s) " +
		"of that header, one per line.\n\nA request template is deleted " +
		"by passing the --delete flag when providing a request name REQ. This will irreversibly remove the request from the project " +
		"entirely.\n\nTo edit an existing request template, provide the name of the request REQ along with attribute-value pairs to set and/or " +
		"any number of the -dXuH flags. -H will always add a new header. Use --remove-header/-r along with the name of the header to remove to " +
		"erase it. Data can be removed by providing a blank value to -d or by providing the -D/--delete-data flag.",
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
