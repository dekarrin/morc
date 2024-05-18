package cmdio

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

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
