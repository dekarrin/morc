package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/rosed"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

var (
	// global flags struct. kept in struct for readability, glub.
	flags = &cliFlags{}
)

const (
	reservedDefaultEnvName = "<DEFAULT>"
)

const (
	// multi-line uses
	annotationKeyHelpUsages = "morc_help_usages"
)

// CUSTOM HELP AND WRAPPING:
//
// - Replace all calls to cmd.Long in help template with call to func that gets
// Long help by first word in Usage. (func getLongHelp(cmd *cobra) which just calls
// a pre-reg func in a global map of func).
// - For all existing Long help, move to new func in global map.
// - Add getLongHelp binding to cobra Template funcs.
// - Ensure getLongHelp calls wrap if func doesn't do it itself (include whether
// pre-wrapped in func registration).

func init() {
	cobra.AddTemplateFunc("wrapFlags", wrappedFlagUsages)
	cobra.AddTemplateFunc("longHelp", getLongHelp)
	cobra.AddTemplateFunc("longUsages", longHelpUsageLines)
}

type longHelp struct {
	fn              func() string
	resultIsWrapped bool
}

var (
	customFormattedCommandDescriptions = map[string]longHelp{}
)

func getLongHelp(cmd *cobra.Command) string {
	if long, ok := customFormattedCommandDescriptions[cmd.Name()]; ok {
		res := long.fn()
		if !long.resultIsWrapped {
			return wrapTerminalText(res)
		}
		return res
	}

	// otherwise,
	return wrapTerminalText(cmd.Long)
}

func wrappedFlagUsages(flagset *pflag.FlagSet) string {
	w := getWrapWidth()
	return flagset.FlagUsagesWrapped(w)
}

func longHelpUsageLines(cmd *cobra.Command) []string {
	usages, ok := cmd.Annotations[annotationKeyHelpUsages]
	if !ok {
		return []string{cmd.UseLine()}
	}

	prefix := ""
	if cmd.HasParent() {
		prefix = cmd.Parent().CommandPath() + " "
	}

	lines := []string{}
	for _, usage := range strings.Split(usages, "\n") {
		usage = strings.TrimSpace(usage)
		if usage != "" {
			usage = prefix + usage
		}

		lines = append(lines, usage)
	}

	return lines
}

func wrapTerminalText(s string) string {
	w := getWrapWidth()
	return rosed.
		Edit(s).
		WrapOpts(w, rosed.Options{
			PreserveParagraphs: true,
		}).
		String()
}

// getWrapWidth returns the amount to wrap things to. It will attempt to
// retrieve the current terminal width in characters and return that. If it
// cannot retrieve it, it will return a default width of 80 characters.
func getWrapWidth() int {
	const defaultWidth = 80

	actualWidth, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		actualWidth = defaultWidth
	}

	return actualWidth
}

// usageTemplate is identical to the one used by default (as of cobra@v1.8.0),
// but with the flag usage explicitly set to wrap using the custom
// wrappedFlagUsages func above. This implements the same pattern in code
// suggested by and authored by @jpmcb on GitHub issue #1805 of the cobra
// library. This implementation is adapted from
// https://github.com/vmware-tanzu/community-edition as linked in that issue by
// @jpmcb.
const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}` + usageAfterUseLineTemplate

const usageAfterUseLineTemplate = `{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{wrapFlags .LocalFlags | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{wrapFlags .InheritedFlags | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// helpTemplate is a custom template used for outputting program help. Includes
// entire Usage section due to it not being easily customizable for case where
// it is shown as part of help output.
const helpTemplate = `{{.Short}}

Usage:
{{range longUsages .}}  {{.}}
{{end}}
{{with longHelp .}}{{. | trimTrailingWhitespaces}}{{end}}{{if or .Runnable .HasSubCommands}}` + usageAfterUseLineTemplate + `{{end}}`

func addRequestOutputFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVarP(&flags.BHeaders, "headers", "", false, "(Output flag) Output the headers of the response")
	cmd.PersistentFlags().BoolVarP(&flags.BCaptures, "captures", "", false, "(Output flag) Output the captures from the response")
	cmd.PersistentFlags().BoolVarP(&flags.BNoBody, "no-body", "", false, "(Output flag) Suppress the output of the response body")
	cmd.PersistentFlags().BoolVarP(&flags.BRequest, "request", "", false, "(Output flag) Output the filled request prior to sending it")
	cmd.PersistentFlags().StringVarP(&flags.Format, "format", "f", "pretty", "(Output flag) Set output format. `FMT` must be one of 'pretty', 'line', or 'sr')")
}

func gatherRequestOutputFlags() (morc.OutputControl, error) {
	oc := morc.OutputControl{}

	// check format
	switch strings.ToLower(flags.Format) {
	case "pretty":
		oc.Format = morc.FormatPretty
	case "sr":
		oc.Format = morc.FormatLine

		// check if user is trying to turn on things that aren't allowed
		if flags.BRequest || flags.BHeaders || flags.BNoBody || flags.BCaptures {
			return oc, fmt.Errorf("format 'sr' only allows status line and response body; use format 'line' for control over output")
		}
	case "line":
		oc.Format = morc.FormatLine
	default:
		return oc, fmt.Errorf("invalid format %q; must be one of pretty, line, or sr", flags.Format)
	}

	oc.Request = flags.BRequest
	oc.Headers = flags.BHeaders
	oc.Captures = flags.BCaptures
	oc.SuppressResponseBody = flags.BNoBody

	return oc, nil
}

// if set, will override loading project from disk.
var (
	projReader io.Reader
	projWriter io.Writer

	histReader io.Reader
	histWriter io.Writer

	seshReader io.Reader
	seshWriter io.Writer
)

func readProject(filename string, all bool) (morc.Project, error) {
	if projReader != nil {
		return morc.LoadProject(projReader, seshReader, histReader)
	}
	return morc.LoadProjectFromDisk(filename, all)
}

func writeProject(p morc.Project, all bool) error {
	if projWriter != nil {
		err := p.Dump(projWriter)
		if err != nil {
			return fmt.Errorf("persist project: %w", err)
		}

		if all {
			if p.Config.SessionFSPath() != "" && seshWriter != nil {
				if err := p.Session.Dump(seshWriter); err != nil {
					return fmt.Errorf("persist session: %w", err)
				}
			}

			if p.Config.HistoryFSPath() != "" && histWriter != nil {
				if err := p.DumpHistory(histWriter); err != nil {
					return fmt.Errorf("persist history: %w", err)
				}
			}
		}
		return nil
	}

	return p.PersistToDisk(all)
}

func writeHistory(p morc.Project) error {
	if histWriter != nil {
		return p.DumpHistory(histWriter)
	}

	return p.PersistHistoryToDisk()
}

func writeSession(p morc.Project) error {
	if seshWriter != nil {
		return p.Session.Dump(histWriter)
	}

	return p.PersistSessionToDisk()
}

type optional[E any] struct {
	set bool
	v   E
}

func (o optional[E]) Or(v E) E {
	if o.set {
		return o.v
	}
	return v
}

type optionalC[E comparable] optional[E]

// Is returns whether the optional is validly set to v. Shorthand for
// o.set && o.v == v.
func (o optionalC[E]) Is(v E) bool {
	return o.set && o.v == v
}

func (o optionalC[E]) Or(v E) E {
	return (optional[E](o)).Or(v)
}

func parseOnOff(s string) (bool, error) {
	up := strings.ToUpper(s)
	if up == "ON" || up == "1" || up == "ENABLE" || up == "TRUE" || up == "T" || up == "YES" || up == "Y" {
		return true, nil
	}

	if up == "OFF" || up == "0" || up == "DISABLE" || up == "FALSE" || up == "F" || up == "NO" || up == "N" {
		return false, nil
	}

	return false, fmt.Errorf("invalid value %q; must be ON or OFF (case-insensitive)", s)
}

// TODO: probs betta off as struct with constants for type and special type for
// when name is set.
type envSelection struct {
	useName    string
	useCurrent bool
	useDefault bool
	useAll     bool
}

func (es envSelection) IsSpecified() bool {
	return es.useName != "" || es.useCurrent || es.useDefault || es.useAll
}

func (es envSelection) String() string {
	if !es.IsSpecified() {
		return "(not specified)"
	}

	if es.useName != "" {
		return fmt.Sprintf("environment %s", es.useName)
	} else if es.useCurrent {
		return "the current environment"
	} else if es.useDefault {
		return "the default environment"
	} else if es.useAll {
		return "all environments"
	}

	panic("should never happen - no selection is set")
}

// Invoker gathers args and holds definitions for invoked commands. IO and
// project file are always available and must be set before invoking most
// commands.
//
// TODO: use this or delete it.
// type Invoker struct {
// 	io       cmdio.IO
// 	projFile string
// }

type cliFlags struct {

	// ProjectFile is the flag that specifies the project file to use while
	// calling `morc req` or subcommands.
	ProjectFile string

	// New takes as argument the name of the resource being created.
	New string

	// Delete requests the deletion of a resource. It takes as argument the name
	// of the resource being deleted.
	Delete string

	// Get requests an attribute of a resource. It takes the attribute as an
	// argument.
	Get string

	// GetHeader requests the value(s) of the header of a resource. It takes the
	// name of the header as an argument.
	GetHeader string

	// Env specifies environment to apply to.
	Env string

	// Vars is variables, in NAME=VALUE format. Can be specified more than once.
	Vars []string

	// RemoveHeaders is a list of headers to be removed.
	RemoveHeaders []string

	// Name is the name of the resource in question.
	Name string

	// VarName is identical to Name but is named differently for readability.
	VarName string

	// BodyData is the bytes of the body of a request. This is either the bytes
	// of the body directly or a filename prepended with an '@' character.
	BodyData string

	// Headers is a list of headers to be added to the request.
	Headers []string

	// Method is the HTTP method to use for the request.
	Method string

	// URL is the URL to send the request to.
	URL string

	// HistoryFile is the path to a history file. It may contain the special
	// string "::PROJ_DIR::"; if so, it will be interpreted as the current
	// directory of the project file at runtime.
	HistoryFile string

	// SessionFile is the path to a history file. It may contain the special
	// string "::PROJ_DIR::"; if so, it will be interpreted as the current
	// directory of the project file at runtime.
	SessionFile string

	// CookieLifetime is a duration string that specifies the maximum lifetime
	// of recorded Set-Cookie instructions.
	CookieLifetime string

	// RecordHistory is a toggle-string flag that indicates whether history
	// recording should be "ON" or "OFF".
	RecordHistory string

	// RecordCookies  is a toggle-string flag that indicates whether cookie
	// recording should be "ON" or "OFF".
	RecordCookies string

	// WriteStateFile is a flag used in one-off commands that gives the path to
	// a state file to write out cookies and variables to.
	WriteStateFile string

	// ReadStateFile is a flag used in one-off commands that gives the path to a
	// state file to read cookies and variables from.
	ReadStateFile string

	// CaptureVars is a flag used in one-off commands that specifies a variable
	// to capture from the response. It can be specified multiple times.
	CaptureVars []string

	// Spec is a flag that gives the specification for a variable capture.
	Spec string

	// StepRemovals is a flag indicating that the given step index is to be
	// removed. It can be specified multiple times.
	StepRemovals []int

	// StepAdds is a flag indicating that the given request is to be added. It
	// is in format [IDX]:REQ. It can be specified multiple times.
	StepAdds []string

	// StepMoves is a flag indicating that the given step is to be moved to the
	// given index. It is in format FROM:[TO]. It can be specified multiple
	// times.
	StepMoves []string

	// StepReplaces is a flag indicating that the request called at the given
	// step is to be updated to the given request. It is in format IDX:REQ.
	// It can be specified multiple times.
	StepReplaces []string

	// Format is a request output control flag that gives the format of the
	// output.
	Format string

	// BRequest is a request output control switch flag that indicates that the
	// request should be printed in addition to any other output.
	BRequest bool

	// BCaptures is a request output control switch flag that indicates that the
	// captures retrieved from a response should be printed in addition to any
	// other output.
	BCaptures bool

	// BHeaders is a request output control switch flag that indicates that the
	// headers of the response should be printed in addition to any other
	// output.
	BHeaders bool

	// BNoBody is a request output control switch flag that indicates that the
	// body of the response should not be printed.
	BNoBody bool

	// BNoDates is a historical request output control switch flag that
	// indicates that dates of historical events should not be printed when they
	// otherwise would.
	BNoDates bool

	// BInfo is a switch flag that indicates that the requested operation is
	// retrieval of a summary of the resource.
	BInfo bool

	// BEnable is a switch flag that indicates that the requested operation is
	// to enable a feature.
	BEnable bool

	// BDisable is a switch flag that indicates that the requested operation is
	// to disable a feature.
	BDisable bool

	// BClear is a switch flag that indicates that the requested operation is to
	// erase all instances of the applicable type of resource.
	BClear bool

	// BRemoveBody is a switch flag that when set, indicates that the body of
	// the resource is to be removed.
	BRemoveBody bool

	// BForce is a switch flag that indicates that the requested operation
	// should proceed even if it is destructive or leads to a non-pristine
	// state.
	BForce bool

	// BDefault is a switch flag that, when set, indicates that the requested
	// operation should be applied to the default environment.
	BDefault bool

	// BNew is a switch flag that, when set, indicates that a new resource is
	// being created. Resource creations should generally use [New] instead,
	// but this flag is used when the resource is not required to have a name.
	BNew bool

	// BDeleteAll is a switch flag that, when set, indicates that all applicable
	// resources of the given type should be deleted.
	BDeleteAll bool

	// BCurrent is a switch flag that, when set, indicates that the requested
	// operation should be applied to the current environment explicitly.
	BCurrent bool

	// BAll is a switch flag that, when set, indicates that the requested
	// operation should be done with all instances of the applicable resource.
	BAll bool

	// BInsecure is a switch flag that, when set, disables TLS certificate
	// verification, allowing requests to go through even if the server's
	// certificate is invalid.
	BInsecure bool
}
