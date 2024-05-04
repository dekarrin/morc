package commands

import (
	"bufio"
	"fmt"
	"os"

	"github.com/dekarrin/rezi/v2"
	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stateCmd)
}

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Read state data",
	Long:  "Load a file containing state data into memory and print out what it contains in human readable format.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]
		return invokeStateShow(filename)
	},
}

func invokeStateShow(filename string) error {
	// open a buffered reader on the file
	fRaw, err := os.Open(filename)
	if err != nil {
		return err
	}

	file := bufio.NewReader(fRaw)

	var state suyac.State
	rzr, err := rezi.NewReader(file, nil)
	if err != nil {
		return err
	}
	if err := rzr.Dec(&state); err != nil {
		return err
	}

	fmt.Printf("State data file %s:\n", filename)
	fmt.Printf("Cookies:\n")
	if len(state.Cookies) == 0 {
		fmt.Printf("(none)\n")
	} else {
		for _, v := range state.Cookies {
			fmt.Printf(" * %s:\n", v.URL)
			for _, cook := range v.Cookies {
				fmt.Printf("   * %s\n", cook.String())
			}
		}
	}
	fmt.Printf("Variables:\n")
	if len(state.Vars) == 0 {
		fmt.Printf("(none)\n")
	} else {
		for k, v := range state.Vars {
			fmt.Printf(" * %s: %s\n", k, v)
		}
	}

	return nil
}
