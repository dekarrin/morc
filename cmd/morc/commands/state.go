package commands

import (
	"bufio"
	"os"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/rezi/v2"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stateCmd)
}

var stateCmd = &cobra.Command{
	Use:   "state FILE",
	Short: "Read oneshot state data",
	Long:  "Load a file containing oneshot state data into memory and print out what it contains in human readable format.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		return invokeStateShow(io, filename)
	},
}

func invokeStateShow(io cmdio.IO, filename string) error {
	// open a buffered reader on the file
	fRaw, err := os.Open(filename)
	if err != nil {
		return err
	}

	file := bufio.NewReader(fRaw)

	var state morc.State
	rzr, err := rezi.NewReader(file, nil)
	if err != nil {
		return err
	}
	if err := rzr.Dec(&state); err != nil {
		return err
	}

	io.Printf("State data file %s:\n", filename)
	io.Printf("Cookies:\n")
	if len(state.Cookies) == 0 {
		io.Printf("(none)\n")
	} else {
		for _, v := range state.Cookies {
			io.Printf(" * %s:\n", v.URL)
			for _, cook := range v.Cookies {
				io.Printf("   * %s\n", cook.String())
			}
		}
	}
	io.Printf("Variables:\n")
	if len(state.Vars) == 0 {
		io.Printf("(none)\n")
	} else {
		for k, v := range state.Vars {
			io.Printf(" * %s: %s\n", k, v)
		}
	}

	return nil
}
