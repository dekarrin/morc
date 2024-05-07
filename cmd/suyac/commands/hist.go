package commands

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"sort"
	"strconv"
	"time"

	"github.com/dekarrin/suyac"
	"github.com/spf13/cobra"
)

var (
	flagHistProjectFile string
	flagHistInfo        bool
	flagHistClear       bool
	flagHistEnable      bool
	flagHistDisable     bool

	flagHistNoDates               bool
	flagHistOutputResponseHeaders bool
	flagHistOutputCaptures        bool
	flagHistOutputRequest         bool
	flagHistSuppressResponseBody  bool
)

func init() {
	histCmd.PersistentFlags().StringVarP(&flagHistProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
	histCmd.PersistentFlags().BoolVarP(&flagHistInfo, "info", "", false, "Print summarizing information about the history")
	histCmd.PersistentFlags().BoolVarP(&flagHistClear, "clear", "", false, "Delete all history entries")
	histCmd.PersistentFlags().BoolVarP(&flagHistEnable, "on", "", false, "Enable history for future requests")
	histCmd.PersistentFlags().BoolVarP(&flagHistDisable, "off", "", false, "Disable history for future requests")

	histCmd.PersistentFlags().BoolVarP(&flagHistNoDates, "no-dates", "", false, "Do not prefix the request with the date of request and response with date of response. Output control option; only used with 'hist ENTRY'")
	histCmd.PersistentFlags().BoolVarP(&flagHistOutputResponseHeaders, "headers", "", false, "Output the headers of the response. Output control option; only used with 'hist ENTRY'")
	histCmd.PersistentFlags().BoolVarP(&flagHistOutputCaptures, "captures", "", false, "Output the captures from the response. Output control option; only used with 'hist ENTRY'")
	histCmd.PersistentFlags().BoolVarP(&flagHistSuppressResponseBody, "no-body", "", false, "Suppress the output of the response body. Output control option; only used with 'hist ENTRY'")
	histCmd.PersistentFlags().BoolVarP(&flagHistOutputRequest, "request", "", false, "Output the filled request prior to sending it. Output control option; only used with 'hist ENTRY'")

	// mark the delete and default flags as mutually exclusive
	histCmd.MarkFlagsMutuallyExclusive("on", "off", "clear", "info")

	rootCmd.AddCommand(histCmd)
}

var histCmd = &cobra.Command{
	Use:     "hist [ENTRY [output-control-opts]] [-F project_file] [--on]|[--off]|[--clear]|[--info]",
	GroupID: "project",
	Short:   "View and perform operations on request template sending history",
	Long:    "With no other arguments, prints out a listing of all summarized entries in the history. If an ENTRY is given by index number from the listing, the exact response as received from the initial send of the template is output. If --on is given, request history is enabled for future requests made by calling suyac send or suyac exec. If --off is given, history is instead disabled, although existing entries are kept. If --info is given, basic info about the history as a whole is output. If --clear is given, all existing history entries are immediately deleted.\n\nHistory only applies to requests created from request templates in a project; one-off requests such as those sent by suyac request or any of the method shorthand versions are not saved in history.",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := histOptions{
			projFile:             flagEnvProjectFile,
			outputRequest:        flagHistOutputRequest,
			outputCaptures:       flagHistOutputCaptures,
			outputHeaders:        flagHistOutputResponseHeaders,
			suppressResponseBody: flagHistSuppressResponseBody,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
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
		} else {
			opts.action = histInfo
		}

		if opts.action != histDetail {
			if flagHistNoDates {
				return fmt.Errorf("--no-dates is only valid when printing history entry details")
			}
			if flagHistOutputRequest {
				return fmt.Errorf("--request is only valid when printing history entry details")
			}
			if flagHistOutputCaptures {
				return fmt.Errorf("--captures is only valid when printing history entry details")
			}
			if flagHistOutputResponseHeaders {
				return fmt.Errorf("--headers is only valid when printing history entry details")
			}
		}

		switch opts.action {
		case histList:
			return invokeHistList(opts)
		case histDetail:
			indexNum, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%q is not a valid history entry index; it must be an integer", args[0])
			}
			return invokeHistDetail(indexNum, opts)
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
	projFile             string
	action               histAction
	outputRequest        bool
	outputCaptures       bool
	outputHeaders        bool
	suppressResponseBody bool
	suppressDates        bool
}

func invokeHistDetail(entryNum int, opts histOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
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

	if opts.outputRequest {
		reqBytes, err := httputil.DumpRequestOut(&hist.Request, true)
		if err != nil {
			return fmt.Errorf("dump request: %w", err)
		}

		fmt.Println("------------------- REQUEST -------------------")
		fmt.Println(string(reqBytes))

		if hist.Request.Body == nil || hist.Request.Body == http.NoBody {
			fmt.Println("(no request body)")
		}

		fmt.Println("----------------- END REQUEST -----------------")
	}

	if opts.outputCaptures {
		fmt.Println("----------------- VAR CAPTURES ----------------")

		capNames := []string{}
		for k := range hist.Captures {
			capNames = append(capNames, k)
		}

		sort.Strings(capNames)

		for _, k := range capNames {
			v := hist.Captures[k]
			fmt.Printf("%s: %s\n", k, v)
		}

		fmt.Println("-----------------------------------------------")
	}

	resp := hist.Response

	fmt.Printf("%s %s\n", resp.Proto, resp.Status)

	if opts.outputHeaders {

		fmt.Println("------------------- HEADERS -------------------")

		// alphabetize the headers
		keys := make([]string, 0, len(resp.Header))
		for k := range resp.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			vals := resp.Header[k]
			for _, v := range vals {
				fmt.Printf("%s: %s\n", k, v)
			}
		}

		fmt.Println("-----------------------------------------------")
	}

	if !opts.suppressResponseBody {
		if resp.Body != nil && resp.Body != http.NoBody {
			entireBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("read response body: %w", err)
			}

			fmt.Println(string(entireBody))
		} else {
			fmt.Println("(no response body)")
		}
	}

	return nil
}

func invokeHistOn(opts histOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.Config.RecordHistory = true

	return p.PersistToDisk(false)
}

func invokeHistOff(opts histOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.Config.RecordHistory = false

	return p.PersistToDisk(false)
}

func invokeHistClear(opts histOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.History = nil

	return p.PersistHistoryToDisk()
}

func invokeHistInfo(opts histOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
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
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if len(p.History) == 0 {
		fmt.Println("(no history entries).")
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
