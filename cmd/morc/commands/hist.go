package commands

import (
	"fmt"
	"strconv"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var histCmd = &cobra.Command{
	Use: "hist [ENTRY]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"hist\n" +
			"hist ENTRY [output-flags]\n" +
			"hist [--on | --off | --clear | --info]",
	},
	GroupID: "project",
	Short:   "View and perform operations on request template sending history",
	Long: "With no other arguments, prints out a listing of all summarized entries in the history. If an ENTRY is " +
		"given by index number from the listing, the exact response as received from the original send of the " +
		"template is printed. If --on is given, request history is enabled for future requests made by calling morc " +
		"send or morc exec. If --off is given, history is instead disabled, although existing entries are kept. If " +
		"--info is given, basic info about the history as a whole is output. If --clear is given, all existing " +
		"history entries are immediately deleted.\n\n" +
		"History only applies to requests created from request templates in a project; one-off requests such as those " +
		"sent by 'morc oneoff' or any of the method shorthand versions are not saved in history.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args histArgs
		if err := parseHistArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		io.Quiet = flags.BQuiet

		switch args.action {
		case histActionList:
			return invokeHistList(io, args.projFile)
		case histActionDetail:
			return invokeHistDetail(io, args.projFile, args.entry, args.outputCtrl, args.noDates)
		case histActionInfo:
			return invokeHistInfo(io, args.projFile)
		case histActionClear:
			return invokeHistClear(io, args.projFile)
		case histActionEnable:
			return invokeHistOn(io, args.projFile)
		case histActionDisable:
			return invokeHistOff(io, args.projFile)
		default:
			panic(fmt.Sprintf("unhandled hist action %q", args.action))
		}
	},
}

func init() {
	histCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	histCmd.PersistentFlags().BoolVarP(&flags.BInfo, "info", "", false, "Print summarizing information about the history")
	histCmd.PersistentFlags().BoolVarP(&flags.BClear, "clear", "", false, "Delete all history entries")
	histCmd.PersistentFlags().BoolVarP(&flags.BEnable, "on", "", false, "Enable history for future requests")
	histCmd.PersistentFlags().BoolVarP(&flags.BDisable, "off", "", false, "Disable history for future requests")
	histCmd.PersistentFlags().BoolVarP(&flags.BNoDates, "no-dates", "", false, "(Output flag) Do not prefix the request with the date of request and response with date of response. Only used with 'hist ENTRY'")
	histCmd.PersistentFlags().BoolVarP(&flags.BQuiet, "quiet", "q", false, "Suppress all unnecessary output.")

	// mark the delete and default flags as mutually exclusive
	histCmd.MarkFlagsMutuallyExclusive("on", "off", "clear", "info")

	addRequestOutputFlags(histCmd)

	rootCmd.AddCommand(histCmd)
}

func invokeHistDetail(io cmdio.IO, projFile string, entry int, reqOC morc.OutputControl, noDates bool) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if entry < 0 {
		return fmt.Errorf("entry number must be positive")
	}
	if entry >= len(p.History) {
		return fmt.Errorf("can't get entry %d; %d is the highest entry available", entry, len(p.History)-1)
	}

	hist := p.History[entry]

	io.Printf("Request template: %s\n", hist.Template)

	if !noDates {
		io.Printf("Request sent:          %s\n", hist.ReqTime.Format(time.RFC3339))
		io.Printf("Response received:     %s\n", hist.RespTime.Format(time.RFC3339))
		io.Printf("Total round-trip time: %s\n", hist.RespTime.Sub(hist.ReqTime))
	}

	reqOC.Writer = io.Out
	if err := morc.OutputRequest(hist.Request, reqOC); err != nil {
		return err
	}

	if err := morc.OutputResponse(hist.Response, hist.Captures, reqOC); err != nil {
		return err
	}

	return nil
}

func invokeHistOn(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if p.Config.HistFile == "" {
		p.Config.HistFile = morc.DefaultHistoryPath
		io.PrintErrf("no history file configured; defaulting to " + p.Config.HistoryFSPath())
	}

	p.Config.RecordHistory = true

	if err := writeProject(p, false); err != nil {
		return err
	}

	io.PrintLoudf("History enabled")

	return nil
}

func invokeHistOff(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	p.Config.RecordHistory = false

	if err := writeProject(p, false); err != nil {
		return err
	}

	io.PrintLoudf("History disabled")

	return nil
}

func invokeHistClear(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	p.History = nil

	if err := writeHistory(p); err != nil {
		return err
	}

	io.PrintLoudf("History cleared")

	return nil
}

func invokeHistInfo(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if p.Config.HistFile == "" {
		io.Println("Project is not configured to use a history file")
		return nil
	}

	entrySuffix := "ies"
	if len(p.History) == 1 {
		entrySuffix = "y"
	}

	io.Printf("%d entr%s in %s\n", len(p.History), entrySuffix, p.Config.HistoryFSPath())
	io.Println()
	if p.Config.RecordHistory {
		io.Println("History is ON")
	} else {
		io.Println("History is OFF")
	}

	return nil
}

func invokeHistList(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if len(p.History) == 0 {
		io.PrintLoudln("(no history)")
		return nil
	}

	// layout:
	// 0: 5/25/1993 12:34:56 PM - get-google - GET /api/v1/thing - 200 OK - 1.2s

	for i, h := range p.History {
		io.Printf(
			"%d: %s - %s - %s %s - %s - %s\n",
			i,
			h.ReqTime.Format(time.RFC3339),
			h.Template,
			h.Request.Method,
			h.Request.URL,
			h.Response.Status,
			h.RespTime.Sub(h.ReqTime),
		)
	}

	return nil
}

type histArgs struct {
	projFile string
	action   histAction

	entry      int
	outputCtrl morc.OutputControl
	noDates    bool
}

func parseHistArgs(cmd *cobra.Command, posArgs []string, args *histArgs) error {
	args.projFile = projPathFromFlagsOrFile(cmd)

	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseHistActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	switch args.action {
	case histActionList:
		// no additional args to parse
	case histActionDetail:
		args.entry, err = strconv.Atoi(posArgs[0])
		if err != nil {
			return fmt.Errorf("%q is not a valid history entry index; it must be an integer", posArgs[0])
		}

		args.outputCtrl, err = gatherRequestOutputFlags(cmd)
		if err != nil {
			return err
		}

		args.noDates = flags.BNoDates
	case histActionInfo, histActionClear, histActionEnable, histActionDisable:
		// no additional args to parse
	default:
		panic(fmt.Sprintf("unhandled hist action %q", args.action))
	}

	return nil
}

func parseHistActionFromFlags(cmd *cobra.Command, posArgs []string) (histAction, error) {
	// mutual exclusions enforced by cobra (and therefore we do not check them here):
	// * --on, --off, --clear, --info

	f := cmd.Flags()

	if f.Changed("on") {
		if len(posArgs) > 0 {
			return histActionEnable, fmt.Errorf("--on cannot be used with positional argument %q", posArgs[0])
		}
		if requestOutputFlagIsPresent(cmd) {
			return histActionEnable, fmt.Errorf("cannot use output flags with --on")
		}
		return histActionEnable, nil
	} else if f.Changed("off") {
		if len(posArgs) > 0 {
			return histActionDisable, fmt.Errorf("--off cannot be used with positional argument %q", posArgs[0])
		}
		if requestOutputFlagIsPresent(cmd) {
			return histActionEnable, fmt.Errorf("cannot use output flags with --off")
		}
		return histActionDisable, nil
	} else if f.Changed("clear") {
		if len(posArgs) > 0 {
			return histActionClear, fmt.Errorf("--clear cannot be used with positional argument %q", posArgs[0])
		}
		if requestOutputFlagIsPresent(cmd) {
			return histActionEnable, fmt.Errorf("cannot use output flags with --clear")
		}
		return histActionClear, nil
	} else if f.Changed("info") {
		if len(posArgs) > 0 {
			return histActionInfo, fmt.Errorf("--info cannot be used with positional argument %q", posArgs[0])
		}
		if requestOutputFlagIsPresent(cmd) {
			return histActionEnable, fmt.Errorf("cannot use output flags with --info")
		}
		return histActionInfo, nil
	}

	if len(posArgs) == 0 {
		if requestOutputFlagIsPresent(cmd) {
			return histActionList, fmt.Errorf("output flags are only valid when selecting a history entry to show")
		}
		return histActionList, nil
	} else if len(posArgs) == 1 {
		return histActionDetail, nil
	} else {
		return histActionList, fmt.Errorf("unknown positional argument %q", posArgs[1])
	}
}

func requestOutputFlagIsPresent(cmd *cobra.Command) bool {
	f := cmd.Flags()

	return f.Changed("request") || f.Changed("captures") || f.Changed("headers") || f.Changed("no-body") || f.Changed("flags") || f.Changed("no-dates")
}

type histAction int

const (
	histActionList histAction = iota
	histActionDetail
	histActionInfo
	histActionClear
	histActionEnable
	histActionDisable
)
