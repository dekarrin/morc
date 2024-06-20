package commands

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

var cookiesCmd = &cobra.Command{
	Use: "cookies",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"cookies [--url URL]\n" +
			"cookies [--on | --off | --clear | --info]",
	},
	GroupID: "project",
	Short:   "View and perform operations on stored cookies",
	Long: "With no other arguments, prints out a listing of all cookies recorded from Set-Cookie headers. If " +
		"--url is given, only cookies that would be set on requests that that URL are printed. If --on is given, " +
		"cookie recording is enabled for future requests made by calling morc send or morc exec. If --off is given, " +
		"cookie recording is instead disabled, although existing cookies are kept until they expire. If --info is " +
		"given, basic info about the cookie store as a whole is output. If --clear is given, existing cookies are " +
		"immediately deleted.\n\n" +
		"Cookie recording only applies to requests created from request templates in a project; one-off requests " +
		"such as those sent by 'morc oneoff' or any of the method shorthand versions will not have their cookies " +
		"associated with the project.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args cookiesArgs
		if err := parseCookiesArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch args.action {
		case cookiesActionList:
			return invokeCookiesList(io, args.projFile, args.url)
		case cookiesActionInfo:
			return invokeCookiesInfo(io, args.projFile)
		case cookiesActionClear:
			return invokeCookiesClear(io, args.projFile)
		case cookiesActionEnable:
			return invokeCookiesOn(io, args.projFile)
		case cookiesActionDisable:
			return invokeCookiesOff(io, args.projFile)
		default:
			panic(fmt.Sprintf("unhandled cookies action %q", args.action))
		}
	},
}

func init() {
	cookiesCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	cookiesCmd.PersistentFlags().BoolVarP(&flags.BInfo, "info", "", false, "Print summarizing information about stored cookies")
	cookiesCmd.PersistentFlags().BoolVarP(&flags.BClear, "clear", "", false, "Delete all cookies")
	cookiesCmd.PersistentFlags().BoolVarP(&flags.BEnable, "on", "", false, "Enable cookie recording for future requests")
	cookiesCmd.PersistentFlags().BoolVarP(&flags.BDisable, "off", "", false, "Disable cookie recording for future requests")
	cookiesCmd.PersistentFlags().StringVarP(&flags.URL, "url", "u", "", "Get cookies that would only be set on the given URL")

	// mark the delete and default flags as mutually exclusive
	cookiesCmd.MarkFlagsMutuallyExclusive("on", "off", "clear", "info", "url")

	rootCmd.AddCommand(cookiesCmd)
}

func invokeCookiesOn(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if p.Config.SeshFile == "" {
		p.Config.HistFile = morc.DefaultSessionPath
		io.PrintErrf("no session file configured; defaulting to " + p.Config.SessionFSPath())
	}

	p.Config.RecordSession = true

	if err := writeProject(p, false); err != nil {
		return err
	}

	io.PrintLoudf("Cookie recording enabled")

	return nil
}

func invokeCookiesOff(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	p.Config.RecordSession = false

	if err := writeProject(p, false); err != nil {
		return err
	}

	io.PrintLoudf("Cookie recording disabled")

	return nil
}

func invokeCookiesClear(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	p.Session.Cookies = nil

	if err := writeSession(p); err != nil {
		return err
	}

	io.PrintLoudf("Cookies cleared")

	return nil
}

func invokeCookiesInfo(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if p.Config.SeshFile == "" {
		io.Println("Project is not configured to use a session file")
		return nil
	}

	// get total number of domains cookies are set on
	countByDomain := map[string]struct{}{}
	totalCount := 0
	for _, c := range p.Session.Cookies {
		u := c.URL.String()

		if _, ok := countByDomain[u]; !ok {
			countByDomain[u] = struct{}{}
		}

		totalCount += len(c.Cookies)
	}

	domainS := "s"
	totalS := "s"

	if len(countByDomain) == 1 {
		domainS = ""
	}

	if totalCount == 1 {
		totalS = ""
	}

	io.Printf("%d cookie%s across %d domain%s in %s\n", totalCount, totalS, len(countByDomain), domainS, p.Config.SessionFSPath())
	io.Println()
	if p.Config.RecordSession {
		io.Println("Cookie recording is ON")
	} else {
		io.Println("Cookie recording is OFF")
	}

	return nil
}

func invokeCookiesList(io cmdio.IO, projFile string, url *url.URL) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if len(p.Session.Cookies) == 0 {
		io.Println("(no cookies)")
		return nil
	}

	if url != nil {
		// list only cookies that would be set on the given URL

		cookies := p.CookiesForURL(url)

		if len(cookies) == 0 {
			io.Println("(no cookies)")
			return nil
		}

		for _, c := range cookies {
			io.Printf("%s\n", c.String())
		}
	} else {
		// list them all
		cookiesByDomain := map[string][]morc.SetCookiesCall{}
		domains := []string{}
		for _, c := range p.Session.Cookies {
			u := c.URL.String()

			if _, ok := cookiesByDomain[u]; !ok {
				domains = append(domains, u)
				cookiesByDomain[u] = []morc.SetCookiesCall{}
			}

			dList := cookiesByDomain[u]
			dList = append(dList, c)
			cookiesByDomain[u] = dList
		}
		sort.Strings(domains)

		for i, d := range domains {
			io.Printf("%s:\n", d)
			for _, call := range cookiesByDomain[d] {
				for _, c := range call.Cookies {
					io.Printf("%s %s\n", call.Time.Format(time.RFC3339), c.String())
				}
			}

			if i < len(domains)-1 {
				io.Println()
			}
		}
	}

	return nil
}

type cookiesArgs struct {
	projFile string
	action   cookiesAction
	url      *url.URL
}

func parseCookiesArgs(cmd *cobra.Command, posArgs []string, args *cookiesArgs) error {
	args.projFile = flags.ProjectFile

	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseCookiesActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	switch args.action {
	case cookiesActionList:
		// pick up
		if flags.URL != "" {
			lowerURL := strings.ToLower(flags.URL)
			if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
				flags.URL = "http://" + flags.URL
			}
			u, err := url.Parse(flags.URL)
			if err != nil {
				return fmt.Errorf("invalid URL: %w", err)
			}
			args.url = u
		}
	case cookiesActionInfo, cookiesActionClear, cookiesActionEnable, cookiesActionDisable:
		// no additional args to parse
	default:
		panic(fmt.Sprintf("unhandled cookies action %q", args.action))
	}

	return nil
}

func parseCookiesActionFromFlags(cmd *cobra.Command, _ []string) (cookiesAction, error) {
	// mutual exclusions enforced by cobra (and therefore we do not check them here):
	// * --on, --off, --clear, --info, --url.

	f := cmd.Flags()

	if f.Changed("on") {
		return cookiesActionEnable, nil
	} else if f.Changed("off") {
		return cookiesActionDisable, nil
	} else if f.Changed("clear") {
		return cookiesActionClear, nil
	} else if f.Changed("info") {
		return cookiesActionInfo, nil
	}

	return cookiesActionList, nil
}

type cookiesAction int

const (
	cookiesActionList cookiesAction = iota
	cookiesActionInfo
	cookiesActionClear
	cookiesActionEnable
	cookiesActionDisable
)
