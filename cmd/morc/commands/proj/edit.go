package proj

import (
	"fmt"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

var (
	flagEditName           string
	flagEditCookieLifetime string
	flagEditSessionFile    string
	flagEditHistoryFile    string
	flagEditCookiesOn      bool
	flagEditCookiesOff     bool
	flagEditHistoryOn      bool
	flagEditHistoryOff     bool
)

func init() {
	editCmd.PersistentFlags().StringVarP(&flagEditName, "name", "n", "", "Change the name of the project.")
	editCmd.PersistentFlags().StringVarP(&flagEditHistoryFile, "history", "H", "", "Set history file to `PATH`. Does not affect whether history is actually recorded; use --hist-on and --hist-off for that. If the special string '"+morc.ProjDirVar+"' is in the path given, it is replaced with the relative directory of the project file whenever morc is executed.")
	editCmd.PersistentFlags().StringVarP(&flagEditSessionFile, "session", "S", "", "Set session file to `PATH`. Does not affect whether session data (cookies) is actually recorded; use --cookies-on and --cookies-off for that.\nIf the special string '"+morc.ProjDirVar+"' is in the path given, it is replaced with the relative directory of the project file whenever morc is executed.")
	editCmd.PersistentFlags().StringVarP(&flagEditCookieLifetime, "cookie-lifetime", "C", "24h", "Set the lifetime of recorded Set-Cookie calls in notation like \"24h\" or \"1h30m\". If set to 0 or less, will be interpreted as 24h. Altering this will immediately apply an eviction check to all current cookies; this may result in some being purged.")
	editCmd.PersistentFlags().BoolVar(&flagEditCookiesOn, "cookies-on", false, "Turn on recording of cookies. Equivalent to 'morc cookies on'.")
	editCmd.PersistentFlags().BoolVar(&flagEditCookiesOff, "cookies-off", false, "Turn off recording of cookies. Equivalent to 'morc cookies off'.")
	editCmd.PersistentFlags().BoolVar(&flagEditHistoryOn, "hist-on", false, "Turn on recording of history. Equivalent to 'morc hist on'.")
	editCmd.PersistentFlags().BoolVar(&flagEditHistoryOff, "hist-off", false, "Turn off recording of history. Equivalent to 'morc hist off'.")

	editCmd.MarkFlagsMutuallyExclusive("cookies-on", "cookies-off")
	editCmd.MarkFlagsMutuallyExclusive("hist-on", "hist-off")

	RootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit [-F project_file] [--name NAME] [-H PATH] [-S PATH] [-C DURATION] [--cookies-on|--cookies-off] [--hist-on|--hist-off]",
	Short: "Edit properties of the project",
	Long:  "Edit properties of the project. One or more flags must be set.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := editOptions{
			projFile: commonflags.ProjectFile,
		}
		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// use Changed to detect setting to the empty string

		if cmd.Flags().Changed("name") {
			opts.newName = optional[string]{set: true, v: flagEditName}
		}

		if cmd.Flags().Changed("history") {
			opts.histFile = optional[string]{set: true, v: flagEditHistoryFile}
		}

		if cmd.Flags().Changed("session") {
			opts.seshFile = optional[string]{set: true, v: flagEditSessionFile}
		}

		if cmd.Flags().Changed("cookie-lifetime") {
			cl, err := time.ParseDuration(flagEditCookieLifetime)
			if err != nil {
				return fmt.Errorf("invalid cookie lifetime duration: %w", err)
			}
			opts.cookieLifetime = optional[time.Duration]{set: true, v: cl}
		}

		// okay, back to normal arg checks
		if flagEditCookiesOn {
			opts.cookies = optional[bool]{set: true, v: true}
		} else if flagEditCookiesOff {
			opts.cookies = optional[bool]{set: true, v: false}
		}

		if flagEditHistoryOn {
			opts.hist = optional[bool]{set: true, v: true}
		} else if flagEditHistoryOff {
			opts.hist = optional[bool]{set: true, v: false}
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true

		return invokeProjEdit(opts)
	},
}

type optional[E comparable] struct {
	set bool
	v   E
}

// Is returns whether the optional is validly set to v. Shorthand for
// o.set && o.v == v.
func (o optional[E]) Is(v E) bool {
	return o.set && o.v == v
}

type editOptions struct {
	projFile       string
	newName        optional[string]
	hist           optional[bool]
	cookies        optional[bool]
	seshFile       optional[string]
	histFile       optional[string]
	cookieLifetime optional[time.Duration]
}

func (eo editOptions) changesFilePaths() bool {
	return eo.seshFile.set || eo.histFile.set
}

func invokeProjEdit(opts editOptions) error {
	// if either the history file or session file are altered, or if cookie
	// lifetime is altered, we need to load the current files to mutate or
	// copy the data
	modifyAllFiles := opts.changesFilePaths() || opts.cookieLifetime.set

	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, modifyAllFiles)
	if err != nil {
		return err
	}

	if opts.newName.set {
		p.Name = opts.newName.v
	}

	if opts.histFile.set {
		if opts.histFile.v == "" {
			// set to empty string effectively disables history, so we cannot do
			// this if history is enabled or if it is about to be.

			// history is on and not turning it off
			if p.Config.RecordHistory && !opts.hist.Is(false) {
				return fmt.Errorf("cannot set history file to empty string: history recording must be disabled first")
			}

			// history is off and turning it on
			if !p.Config.RecordHistory && opts.hist.Is(true) {
				return fmt.Errorf("cannot set history file to empty string when passing --hist-on")
			}

			// otherwise, it is safe to set it.
		}
		p.Config.HistFile = opts.histFile.v
	}

	if opts.seshFile.set {
		if opts.seshFile.v == "" {
			// set to empty string effectively disables session saving, so we
			// cannot do this if cookie saving is enabled or if it is about to
			// be.

			// cookies are on and not turning it off
			if p.Config.RecordSession && !opts.cookies.Is(false) {
				return fmt.Errorf("cannot set session file to empty string: cookie recording must be disabled first")
			}

			// cookies are off and turning it on
			if !p.Config.RecordSession && opts.cookies.Is(true) {
				return fmt.Errorf("cannot set session file to empty string when passing --cookies-on")
			}

			// otherwise, it is safe to set it.
		}
		p.Config.SeshFile = opts.seshFile.v
	}

	if opts.cookieLifetime.set {
		p.Config.CookieLifetime = opts.cookieLifetime.v
		p.EvictOldCookies()
	}

	if opts.hist.set {
		// enabling is not allowed if the history file is unset
		if p.Config.HistFile == "" && opts.hist.Is(true) {
			return fmt.Errorf("cannot enable history recording: no history file set")
		}

		p.Config.RecordHistory = opts.hist.v
	}

	if opts.cookies.set {
		// enabling is not allowed if the session file is unset
		if p.Config.SeshFile == "" && opts.cookies.Is(true) {
			return fmt.Errorf("cannot enable cookie recording: no session file set")
		}

		p.Config.RecordSession = opts.cookies.v
	}

	return p.PersistToDisk(modifyAllFiles)
}
