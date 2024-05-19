package commands

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
)

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
