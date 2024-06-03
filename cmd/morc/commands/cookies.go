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

var (
	flagCookiesProjectFile string
	flagCookiesInfo        bool
	flagCookiesClear       bool
	flagCookiesEnable      bool
	flagCookiesDisable     bool
	flagCookiesURL         string
)

func init() {
	cookiesCmd.PersistentFlags().StringVarP(&flagCookiesProjectFile, "project_file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)
	cookiesCmd.PersistentFlags().BoolVarP(&flagCookiesInfo, "info", "", false, "Print summarizing information about stored cookies")
	cookiesCmd.PersistentFlags().BoolVarP(&flagCookiesClear, "clear", "", false, "Delete all cookies")
	cookiesCmd.PersistentFlags().BoolVarP(&flagCookiesEnable, "on", "", false, "Enable cookie recording for future requests")
	cookiesCmd.PersistentFlags().BoolVarP(&flagCookiesDisable, "off", "", false, "Disable cookie recording for future requests")
	cookiesCmd.PersistentFlags().StringVarP(&flagCookiesURL, "url", "u", "", "Get cookies that would only be set on the given URL")

	// mark the delete and default flags as mutually exclusive
	cookiesCmd.MarkFlagsMutuallyExclusive("on", "off", "clear", "info", "url")

	rootCmd.AddCommand(cookiesCmd)
}

var cookiesCmd = &cobra.Command{
	Use: "cookies",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"cookies [-F FILE] [--url URL]\n" +
			"cookies [-F FILE] [--on | --off | --clear | --info]",
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
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := cookiesOptions{
			projFile: flagEnvProjectFile,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
		}

		// parse the URL if given
		if flagCookiesURL != "" {
			lowerURL := strings.ToLower(flagCookiesURL)
			if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
				flagCookiesURL = "http://" + flagCookiesURL
			}
			u, err := url.Parse(flagCookiesURL)
			if err != nil {
				return fmt.Errorf("invalid URL: %w", err)
			}
			opts.url = u
		}

		if flagCookiesInfo {
			opts.action = cookiesInfo
		} else if flagCookiesClear {
			opts.action = cookiesClear
		} else if flagCookiesEnable {
			opts.action = cookiesEnable
		} else if flagCookiesDisable {
			opts.action = cookiesDisable
		} else {
			opts.action = cookiesList
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch opts.action {
		case cookiesList:
			return invokeCookiesList(io, opts)
		case cookiesInfo:
			return invokeCookiesInfo(io, opts)
		case cookiesClear:
			return invokeCookiesClear(io, opts)
		case cookiesEnable:
			return invokeCookiesOn(io, opts)
		case cookiesDisable:
			return invokeCookiesOff(io, opts)
		default:
			panic(fmt.Sprintf("unhandled cookies action %q", opts.action))
		}
	},
}

type cookiesAction int

const (
	cookiesList cookiesAction = iota
	cookiesInfo
	cookiesClear
	cookiesEnable
	cookiesDisable
)

type cookiesOptions struct {
	projFile string
	action   cookiesAction
	url      *url.URL
}

func invokeCookiesOn(io cmdio.IO, opts cookiesOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if p.Config.SeshFile == "" {
		p.Config.HistFile = morc.DefaultSessionPath
		io.PrintErrf("no session file configured; defaulting to " + p.Config.SessionFSPath())
	}

	p.Config.RecordSession = true

	return p.PersistToDisk(false)
}

func invokeCookiesOff(_ cmdio.IO, opts cookiesOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.Config.RecordSession = false

	return p.PersistToDisk(false)
}

func invokeCookiesClear(_ cmdio.IO, opts cookiesOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.Session.Cookies = nil

	return p.PersistSessionToDisk()
}

func invokeCookiesInfo(io cmdio.IO, opts cookiesOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
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

func invokeCookiesList(io cmdio.IO, opts cookiesOptions) error {
	p, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if len(p.Session.Cookies) == 0 {
		io.Println("(no cookies)")
		return nil
	}

	if opts.url != nil {
		// list only cookies that would be set on the given URL

		cookies := p.CookiesForURL(opts.url)

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
