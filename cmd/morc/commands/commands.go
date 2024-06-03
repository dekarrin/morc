package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/rosed"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
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

type requestOutputFlagSet struct {
	Request              bool
	Captures             bool
	Headers              bool
	SuppressResponseBody bool
	Format               string
}

var (
	managedFlagSets = map[string]*requestOutputFlagSet{}
)

func setupRequestOutputFlags(id string, cmd *cobra.Command) {
	if _, ok := managedFlagSets[id]; ok {
		panic("Flag set already exists for " + id)
	}

	flags := &requestOutputFlagSet{}
	managedFlagSets[id] = flags

	cmd.PersistentFlags().BoolVarP(&flags.Headers, "headers", "", false, "Output the headers of the response")
	cmd.PersistentFlags().BoolVarP(&flags.Captures, "captures", "", false, "Output the captures from the response")
	cmd.PersistentFlags().BoolVarP(&flags.SuppressResponseBody, "no-body", "", false, "Suppress the output of the response body")
	cmd.PersistentFlags().BoolVarP(&flags.Request, "request", "", false, "Output the filled request prior to sending it")
	cmd.PersistentFlags().StringVarP(&flags.Format, "format", "f", "pretty", "Output format (pretty, line, sr)")
}

func gatherRequestOutputFlags(id string) (morc.OutputControl, error) {
	flags, ok := managedFlagSets[id]
	if !ok {
		panic("No flag set exists for " + id)
	}

	oc := morc.OutputControl{}

	// check format
	switch strings.ToLower(flags.Format) {
	case "pretty":
		oc.Format = morc.FormatPretty
	case "sr":
		oc.Format = morc.FormatLine

		// check if user is trying to turn on things that aren't allowed
		if flags.Request || flags.Headers || flags.SuppressResponseBody || flags.Captures {
			return oc, fmt.Errorf("format 'sr' only allows status line and response body; use format 'line' for control over output")
		}
	case "line":
		oc.Format = morc.FormatLine
	default:
		return oc, fmt.Errorf("invalid format %q; must be one of pretty, line, or sr", flags.Format)
	}

	oc.Request = flags.Request
	oc.Headers = flags.Headers
	oc.Captures = flags.Captures
	oc.SuppressResponseBody = flags.SuppressResponseBody

	return oc, nil
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

// Invoker gathers args and holds definitions for invoked commands. IO and
// project file are always available and must be set before invoking most
// commands.
//
// TODO: use this or delete it.
// type Invoker struct {
// 	io       cmdio.IO
// 	projFile string
// }
