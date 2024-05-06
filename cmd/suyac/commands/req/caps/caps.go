package caps

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	flagProjectFile string
)

var RootCmd = &cobra.Command{
	Use:     "caps NAME [-F project_file]",
	GroupID: "project",
	Short:   "List variable captures in a request template",
	Long:    "Print a listing of all variable captures that will be attempted on responses to requests made from this template.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := flagProjectFile

		if filename == "" {
			return fmt.Errorf("project file is set to empty string")
		}
		//		return listRequests(filename)
		return nil
	},
}
