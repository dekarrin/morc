package commands

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/dekarrin/morc"
	"github.com/spf13/cobra"
)

var (
	flagHistProjectFile string
	flagHistInfo        bool
	flagHistClear       bool
	flagHistEnable      bool
	flagHistDisable     bool

	flagHistNoDates bool
)

func init() {
	histCmd.PersistentFlags().StringVarP(&flagHistProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	histCmd.PersistentFlags().BoolVarP(&flagHistInfo, "info", "", false, "Print summarizing information about the history")
	histCmd.PersistentFlags().BoolVarP(&flagHistClear, "clear", "", false, "Delete all history entries")
	histCmd.PersistentFlags().BoolVarP(&flagHistEnable, "on", "", false, "Enable history for future requests")
	histCmd.PersistentFlags().BoolVarP(&flagHistDisable, "off", "", false, "Disable history for future requests")

	histCmd.PersistentFlags().BoolVarP(&flagHistNoDates, "no-dates", "", false, "Do not prefix the request with the date of request and response with date of response. Output control option; only used with 'hist ENTRY'")

	// mark the delete and default flags as mutually exclusive
	histCmd.MarkFlagsMutuallyExclusive("on", "off", "clear", "info")

	setupRequestOutputFlags("morc hist", histCmd)

	rootCmd.AddCommand(histCmd)
}

var histCmd = &cobra.Command{
	Use:     "hist [ENTRY [output-control-opts]] [-F project_file] [--on]|[--off]|[--clear]|[--info]",
	GroupID: "project",
	Short:   "View and perform operations on request template sending history",
	Long:    "With no other arguments, prints out a listing of all summarized entries in the history. If an ENTRY is given by index number from the listing, the exact response as received from the initial send of the template is output. If --on is given, request history is enabled for future requests made by calling morc send or morc exec. If --off is given, history is instead disabled, although existing entries are kept. If --info is given, basic info about the history as a whole is output. If --clear is given, all existing history entries are immediately deleted.\n\nHistory only applies to requests created from request templates in a project; one-off requests such as those sent by morc request or any of the method shorthand versions are not saved in history.",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := histOptions{
			projFile: flagEnvProjectFile,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		var err error
		opts.outputCtrl, err = gatherRequestOutputFlags("morc hist")
		if err != nil {
			return err
		}

		if flagHistInfo {
			if len(args) > 0 {
				return fmt.Errorf("cannot use --info when giving an entry number")
			}
			opts.action = histInfo
		} else if flagHistClear {
			if len(args) > 0 {
				return fmt.Errorf("cannot use --clear when giving an entry number")
			}
			opts.action = histClear
		} else if flagHistEnable {
			if len(args) > 0 {
				return fmt.Errorf("cannot use --on when giving an entry number")
			}
			opts.action = histEnable
		} else if flagHistDisable {
			if len(args) > 0 {
				return fmt.Errorf("cannot use --off when giving an entry number")
			}
			opts.action = histDisable
		} else {
			opts.action = histList
		}

		if len(args) > 0 {
			opts.action = histDetail
		}

		if opts.action != histDetail {
			if flagHistNoDates {
				return fmt.Errorf("--no-dates is only valid when printing history entry details")
			}
			if opts.outputCtrl.Request {
				return fmt.Errorf("--request is only valid when printing history entry details")
			}
			if opts.outputCtrl.Captures {
				return fmt.Errorf("--captures is only valid when printing history entry details")
			}
			if opts.outputCtrl.Headers {
				return fmt.Errorf("--headers is only valid when printing history entry details")
			}
			if opts.outputCtrl.SuppressResponseBody {
				return fmt.Errorf("--no-body is only valid when printing history entry details")
			}
		}

		var entryIndex int

		if opts.action == histDetail {
			entryIndex, err = strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%q is not a valid history entry index; it must be an integer", args[0])
			}
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		switch opts.action {
		case histList:
			return invokeHistList(opts)
		case histDetail:
			return invokeHistDetail(entryIndex, opts)
		case histInfo:
			return invokeHistInfo(opts)
		case histClear:
			return invokeHistClear(opts)
		case histEnable:
			return invokeHistOn(opts)
		case histDisable:
			return invokeHistOff(opts)
		default:
			panic(fmt.Sprintf("unhandled hist action %q", opts.action))
		}
	},
}

type histAction int

const (
	// list all environments
	histList histAction = iota
	histDetail
	histInfo
	histClear
	histEnable
	histDisable
)

type histOptions struct {
	projFile      string
	action        histAction
	outputCtrl    morc.OutputControl
	suppressDates bool
}

func invokeHistDetail(entryNum int, opts histOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if entryNum < 0 {
		return fmt.Errorf("entry number must be positive")
	}
	if entryNum >= len(p.History) {
		return fmt.Errorf("can't get entry %d; %d is the highest entry available", entryNum, len(p.History)-1)
	}

	hist := p.History[entryNum]

	fmt.Printf("Request template: %s\n", hist.Template)

	if !opts.suppressDates {
		fmt.Printf("Request sent:          %s\n", hist.ReqTime.Format(time.RFC3339))
		fmt.Printf("Response received:     %s\n", hist.RespTime.Format(time.RFC3339))
		fmt.Printf("Total round-trip time: %s\n", hist.RespTime.Sub(hist.ReqTime))
	}

	if err := morc.OutputRequest(hist.Request, opts.outputCtrl); err != nil {
		return err
	}

	if err := morc.OutputResponse(hist.Response, hist.Captures, opts.outputCtrl); err != nil {
		return err
	}

	return nil
}

func invokeHistOn(opts histOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if p.Config.HistFile == "" {
		p.Config.HistFile = morc.DefaultHistoryPath
		fmt.Fprintf(os.Stderr, "no history file configured; defaulting to "+p.Config.HistoryFSPath())
	}

	p.Config.RecordHistory = true

	return p.PersistToDisk(false)
}

func invokeHistOff(opts histOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.Config.RecordHistory = false

	return p.PersistToDisk(false)
}

func invokeHistClear(opts histOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.History = nil

	return p.PersistHistoryToDisk()
}

func invokeHistInfo(opts histOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if p.Config.HistFile == "" {
		fmt.Println("Project is not configured to use a history file")
		return nil
	}

	entrySuffix := "ies"
	if len(p.History) == 1 {
		entrySuffix = "y"
	}

	fmt.Printf("%d entr%s in %s\n", len(p.History), entrySuffix, p.Config.HistoryFSPath())
	fmt.Println()
	if p.Config.RecordHistory {
		fmt.Println("History is ON")
	} else {
		fmt.Println("History is OFF")
	}

	return nil
}

func invokeHistList(opts histOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if len(p.History) == 0 {
		fmt.Println("(no history)")
		return nil
	}

	// layout:
	// 0: 5/25/1993 12:34:56 PM - get-google - GET /api/v1/thing - 200 OK - 1.2s

	for i, h := range p.History {
		fmt.Printf(
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
