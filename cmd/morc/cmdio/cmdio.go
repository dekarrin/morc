package cmdio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// AttrKey is an identifier for an attribute of some resource, such as the
// config keys of a project or a the properties of a request template. It is
// used for providing standardized output and sorting using
// [OutputLoudEditAttrsResult] and [SortedAttrMapKeys].
type AttrKey interface {
	Human() string
	Name() string
}

// CAttrKey is an AttrKey that also implements comparable.
type CAttrKey interface {
	comparable
	AttrKey
}

var (
	// If HTTPClient is set, it will be used for all requests made by the
	// commands that go through morc.Send. Otherwise, the default client will be
	// used. Generally, this is useful for testing; if it starts getting used
	// for other things, we may need a way of specifying via CLI args instead.
	HTTPClient *http.Client = nil
)

// IO holds the input and output streams for a command, including a settable
// error, output, and input stream. If Out or Err are not set, the std streams
// will be used when calling Println, Printf, or their error variants.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	// Quiet is set to true if the command should not print anything to the
	// output stream for PrintLoud functions. Other Print functions will still
	// print to their repspective streams (out or err).
	Quiet bool
}

// From returns an IO struct taken by retrieving streams from the given command.
func From(cmd *cobra.Command) IO {
	return IO{
		In:  cmd.InOrStdin(),
		Out: cmd.OutOrStdout(),
		Err: cmd.ErrOrStderr(),
	}
}

func (io IO) Println(args ...interface{}) {
	if io.Out == nil {
		fmt.Println(args...)
	} else {
		fmt.Fprintln(io.Out, args...)
	}
}

func (io IO) PrintErrln(args ...interface{}) {
	if io.Err == nil {
		fmt.Fprintln(os.Stderr, args...)
	} else {
		fmt.Fprintln(io.Err, args...)
	}
}

func (io IO) Printf(format string, args ...interface{}) {
	if io.Out == nil {
		fmt.Printf(format, args...)
	} else {
		fmt.Fprintf(io.Out, format, args...)
	}
}

func (io IO) PrintErrf(format string, args ...interface{}) {
	if io.Err == nil {
		fmt.Fprintf(os.Stderr, format, args...)
	} else {
		fmt.Fprintf(io.Err, format, args...)
	}
}

func (io IO) PrintLoudln(args ...interface{}) {
	if !io.Quiet {
		io.Println(args...)
	}
}

func (io IO) PrintLoudf(format string, args ...interface{}) {
	if !io.Quiet {
		io.Printf(format, args...)
	}
}

func (io IO) PrintLoudErrln(args ...interface{}) {
	if !io.Quiet {
		io.PrintErrln(args...)
	}
}

func (io IO) PrintLoudErrf(format string, args ...interface{}) {
	if !io.Quiet {
		io.PrintErrf(format, args...)
	}
}

// extracts map keys in order of some strict ordering slice
func SortedAttrMapKeys[K CAttrKey, V any](m map[K]V, order []K) []K {
	keys := []K{}
	for _, k := range order {
		if _, ok := m[k]; ok {
			keys = append(keys, k)
		}
	}
	return keys
}

// OutputEditAttrsResult prints out the results of an edit operation on a set of
// attributes. It will print out the attributes that were modified and the
// attributes that were not changed.
//
// All output is considered 'loud' and will not be printed if the IO is in quiet
// mode.
//
// Edits that resulted in a mutation will be printed to the output stream and
// edits that did not result in a mutation will be printed to the error stream.
func OutputLoudEditAttrsResult[K CAttrKey](io IO, modifiedVals map[K]interface{}, noChangeVals map[K]interface{}, ordering []K) {
	// if IO is quite, no need to go to the trouble of printing and sorting
	if io.Quiet {
		return
	}

	// create our output
	if len(modifiedVals) > 0 {
		io.PrintLoudf("Set ")

		// get ordering we want
		modKeys := SortedAttrMapKeys(modifiedVals, ordering)

		// turn to slice of output values and let IO handle commas
		setMessages := []string{}
		for _, k := range modKeys {
			v := modifiedVals[k]

			if fmt.Sprintf("%v", v) == "" {
				v = `""`
			}

			setMessages = append(setMessages, fmt.Sprintf("%s to %s", k.Human(), v))
		}

		io.PrintLoudf("%s\n", io.OxfordCommaJoin(setMessages))
	}

	if len(noChangeVals) > 0 {
		// get ordering we want
		noChangeKeys := SortedAttrMapKeys(noChangeVals, ordering)

		// we don't need to do fancy string building because we will simply output
		// each one on its own line
		for _, k := range noChangeKeys {
			v := noChangeVals[k]

			if fmt.Sprintf("%v", v) == "" {
				v = `""`
			}

			io.PrintLoudErrf("No change to %s; already set to %s\n", k.Human(), v)
		}
	}
}

func (io IO) OxfordCommaJoin(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	if len(items) == 2 {
		return items[0] + " and " + items[1]
	}

	// more than 2 items means commas
	var sb strings.Builder
	for i, item := range items {
		if i > 0 {
			sb.WriteString(", ")
		}
		if i+1 == len(items) {
			sb.WriteString("and ")
		}

		sb.WriteString(item)
	}

	return sb.String()
}

func (io IO) OnOrOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

// Count returns a string that automatically pluralizes the given word based on
// whether it is 0 or 1.
//
// If suffixes is not set, it is assumed that the plural is formed by taking
// word and adding "s". If suffixes is set, the first element is used for the
// plural form and the second is used for the singular form.
func (io IO) CountOf(count int, word string, suffixes ...string) string {
	pluralSuf := "s"
	singularSuf := ""

	if len(suffixes) > 0 {
		pluralSuf = suffixes[0]
		if len(suffixes) > 1 {
			singularSuf = suffixes[1]
		}
	}

	plural := word + pluralSuf
	singular := word + singularSuf

	var desc string
	if count == 1 {
		desc = singular
	} else {
		desc = plural
	}

	return fmt.Sprintf("%d %s", count, desc)
}
