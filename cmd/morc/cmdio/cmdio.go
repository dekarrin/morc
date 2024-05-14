package cmdio

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// IO holds the input and output streams for a command, including a settable
// error, output, and input stream. If Out or Err are not set, the std streams
// will be used when calling Println, Printf, or their error variants.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
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
