package commands

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/dekarrin/suyac"
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
	cookiesCmd.PersistentFlags().StringVarP(&flagCookiesProjectFile, "project_file", "F", suyac.DefaultProjectPath, "Use the specified file for project data instead of "+suyac.DefaultProjectPath)
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
	Use:     "cookies [-F project_file] [--url URL] [--on]|[--off]|[--clear]|[--info]",
	GroupID: "project",
	Short:   "View and perform operations on stored cookies",
	Long:    "With no other arguments, prints out a listing of all cookies recorded from Set-Cookie headers. If --url is given, only cookies that would be set on requests that that URL are printed. If --on is given, cookie recording is enabled for future requests made by calling suyac send or suyac exec. If --off is given, cookie recording is instead disabled, although existing cookies are kept until they expire. If --info is given, basic info about the cookie store as a whole is output. If --clear is given, existing cookies are immediately deleted.\n\nCookie recording only applies to requests created from request templates in a project; one-off requests such as those sent by suyac request or any of the method shorthand versions will not have their cookies associated with the project.",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := cookiesOptions{
			projFile: flagEnvProjectFile,
			url:      flagCookiesURL,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file is set to empty string")
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

		switch opts.action {
		case cookiesList:
			return invokeCookiesList(opts)
		case cookiesInfo:
			return invokeCookiesInfo(opts)
		case cookiesClear:
			return invokeCookiesClear(opts)
		case cookiesEnable:
			return invokeCookiesOn(opts)
		case cookiesDisable:
			return invokeCookiesOff(opts)
		default:
			panic(fmt.Sprintf("unhandled cookies action %q", opts.action))
		}
	},
}

type cookiesAction int

const (
	// list all environments
	cookiesList cookiesAction = iota
	cookiesInfo
	cookiesClear
	cookiesEnable
	cookiesDisable
)

type cookiesOptions struct {
	projFile string
	action   cookiesAction
	url      string
}

func invokeCookiesOn(opts cookiesOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if p.Config.SeshFile == "" {
		p.Config.HistFile = suyac.DefaultSessionPath
		fmt.Fprintf(os.Stderr, "no session file configured; defaulting to "+p.Config.SessionFSPath())
	}

	p.Config.RecordSession = true

	return p.PersistToDisk(false)
}

func invokeCookiesOff(opts cookiesOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.Config.RecordSession = false

	return p.PersistToDisk(false)
}

func invokeCookiesClear(opts cookiesOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	p.Session.Cookies = nil

	return p.PersistSessionToDisk()
}

func invokeCookiesInfo(opts cookiesOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if p.Config.SeshFile == "" {
		fmt.Println("Project is not configured to use a session file")
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

	fmt.Printf("%d cookie%s across %d domain%s in %s\n", totalCount, totalS, len(countByDomain), domainS, p.Config.SessionFSPath())
	fmt.Println()
	if p.Config.RecordSession {
		fmt.Println("Cookie recording is ON")
	} else {
		fmt.Println("Cookie recording is OFF")
	}

	return nil
}

func invokeCookiesList(opts cookiesOptions) error {
	p, err := suyac.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	if len(p.Session.Cookies) == 0 {
		fmt.Println("(no cookies)")
		return nil
	}

	cookiesByDomain := map[string][]suyac.SetCookiesCall{}
	domains := []string{}
	for _, c := range p.Session.Cookies {
		u := c.URL.String()

		if _, ok := cookiesByDomain[u]; !ok {
			domains = append(domains, u)
			cookiesByDomain[u] = []suyac.SetCookiesCall{}
		}

		dList := cookiesByDomain[u]
		dList = append(dList, c)
		cookiesByDomain[u] = dList
	}
	sort.Strings(domains)

	for i, d := range domains {
		fmt.Printf("%s:\n", d)
		for _, call := range cookiesByDomain[d] {
			for _, c := range call.Cookies {
				fmt.Printf("%s %s\n", call.Time.Format(time.RFC3339), c.String())
			}
		}

		if i < len(domains)-1 {
			fmt.Println()
		}
	}

	return nil
}
