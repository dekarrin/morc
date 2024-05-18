package proj

import (
	"fmt"
	"strings"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/spf13/cobra"
)

func init() {
	ProjCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use the specified file for project data instead of "+morc.DefaultProjectPath)

	// DO NOT DELETE BELOW UNTIL NOTES ARE MOVED TO MAIN HELP
	// RootCmd.PersistentFlags().StringVarP(&flagEditName, "name", "n", "", "Change the name of the project.")
	// RootCmd.PersistentFlags().StringVarP(&flagProjHistoryFile, "history-file", "H", "", "Show the currently-set path for the history file.\nWhen used with --edit or --new: Set history file to `PATH`. Does not affect whether history is actually recorded; use --hist-on and --hist-off for that. If the special string '"+morc.ProjDirVar+"' is in the path given, it is replaced with the relative directory of the project file whenever morc is executed.")
	// RootCmd.PersistentFlags().StringVarP(&flagProjSessionFile, "session-file", "S", "", "Show the currently-set path for the session file.\nWhen used with --edit or --new: Set session file to `PATH`. Does not affect whether session data (cookies) is actually recorded; use --cookies-on and --cookies-off for that.\nIf the special string '"+morc.ProjDirVar+"' is in the path given, it is replaced with the relative directory of the project file whenever morc is executed.")
	// RootCmd.PersistentFlags().StringVarP(&flagProjCookieLifetime, "cookie-lifetime", "C", "24h", "Get the lifetime of recorded Set-Cookie calls.\nWhen used with --edit or --new: Set the lifetime of recorded Set-Cookie calls to the given duration. `DUR` must be a string in notation like \"24h\" or \"1h30m\". If set to 0 or less, will be interpreted as 24h. Altering this will immediately apply an eviction check to all current cookies; this may result in some being purged.")
	// RootCmd.PersistentFlags().BoolVarP(&flagProjRecordCookies, "cookies", "c", false, "Show whether cookie recording is currently enabled.\nWhen used with --edit or --new: Enable or disable cookie recording. `ON|OFF` must be either the string \"ON\" or \"OFF\", case-insensitive. Equivalent to 'morc cookies --on' or 'morc cookies --off'.")
	// RootCmd.PersistentFlags().BoolVarP(&flagProjRecordHistory, "history", "r", false, "Show whether history recording is currently enabled.\nWhen used with --edit or --new: Enable or disable history recording. `ON|OFF` must be either the string \"ON\" or \"OFF\", case-insensitive. Equivalent to 'morc hist --on' or 'morc hist --off'.")

	// RootCmd.PersistentFlags().BoolVarP(&flagProjEdit, "edit", "", false, "Edit the project. Combine with other options to select item(s) to be modified and give values.")
	ProjCmd.PersistentFlags().BoolVarP(&flagProjNew, "new", "", false, "Create a new project instead of reading/editing one. Combine with other arguments to specify values for the new project.")

	// proj [--edit|--new ITEM=value ITEM=value...] OR proj --thing
	// NOPE
	// proj ITEM, proj ITEM VALUE. --new is the only required option.
}

var ProjCmd = &cobra.Command{
	Use:     "proj [ATTR [VALUE [ATTR2 VALUE2]...]] [-F project_file] [--new]",
	GroupID: "project",
	Short:   "Show or manipulate project attributes and config",
	Long:    "Show the contents of the morc project referred to in the .morc dir in the current directory, or use -F to read a file in another location. If the name of a project attribute is given as an arg, only its current value is printed out. If a second argument is given, then the attribute is set to that value. Multiple pairs of arguments can be given to specify multiple values. If --new is given, a new project file is created instead of editing an existing one.\nThe valid attributes are case-insensitive and are: " + strings.Join(projAttrKeyNames(), ", ") + ".",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := projOptions{
			projFile: commonflags.ProjectFile,
		}

		if opts.projFile == "" {
			return fmt.Errorf("project file cannot be set to empty string")
		}

		// find out our action and do action-specific flag checks
		var getItem projKey

		if len(args) == 0 {
			// either the user wants all the info or new is set and we are making
			// a new project with all defaults.

			if flagProjNew {
				opts.action = projNew
			} else {
				opts.action = projInfo
			}
		} else {
			// begin parsing each of the args

			var curKey projKey
			var err error

			for i, arg := range args {
				if i%2 == 0 {
					// if even, should be an attribute.
					curKey, err = parseProjAttrKey(arg)
					if err != nil {
						return fmt.Errorf("attribute #%d: %w", (i/2)+1, err)
					}

					// do an "already set" check
					setTwice := false
					switch curKey {
					case projKeyName:
						setTwice = opts.name.set
					case projKeyHistFile:
						setTwice = opts.histFile.set
					case projKeySeshFile:
						setTwice = opts.seshFile.set
					case projKeyCookieLifetime:
						setTwice = opts.cookieLifetime.set
					case projKeyCookies:
						setTwice = opts.recordCookies.set
					case projKeyHistory:
						setTwice = opts.recordHistory.set
					}

					if setTwice {
						return fmt.Errorf("%s is set more than once", curKey)
					}
				} else {
					// if odd, it is a value
					switch curKey {
					case projKeyName:
						opts.name = optionalC[string]{set: true, v: arg}
					case projKeyHistFile:
						opts.histFile = optionalC[string]{set: true, v: arg}
					case projKeySeshFile:
						opts.seshFile = optionalC[string]{set: true, v: arg}
					case projKeyCookieLifetime:
						cl, err := time.ParseDuration(arg)
						if err != nil {
							return fmt.Errorf("value for %s: %w", curKey, err)
						}
						opts.cookieLifetime = optionalC[time.Duration]{set: true, v: cl}
					case projKeyCookies:
						isOn, err := parseOnOff(arg)
						if err != nil {
							return fmt.Errorf("value for %s: %w", curKey, err)
						}
						opts.recordCookies = optionalC[bool]{set: true, v: isOn}
					case projKeyHistory:
						isOn, err := parseOnOff(arg)
						if err != nil {
							return fmt.Errorf("value for %s: %w", curKey, err)
						}
						opts.recordHistory = optionalC[bool]{set: true, v: isOn}
					}
				}
			}

			// now that we are done, do an arg-count check and use it to set
			// action.
			// doing AFTER parsing so that we can give a betta error message if
			// missing last value
			if len(args) == 1 {
				// that's fine, we just want to get the one item
				opts.action = projGet
				getItem = curKey
			} else if len(args)%2 != 0 {
				return fmt.Errorf("%s is missing a value", curKey)
			} else {
				if flagProjNew {
					opts.action = projNew
				} else {
					opts.action = projEdit
				}
			}

		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch opts.action {
		case projInfo:
			return invokeProjShow(io, opts)
		case projGet:
			return invokeProjGet(io, getItem, opts)
		case projNew:
			return invokeProjNew(io, opts.name.v, opts)
		case projEdit:
			return invokeProjEdit(io, opts)
		default:
			panic(fmt.Sprintf("unhandled proj action %q", opts.action))
		}
	},
}

type optional[E any] struct {
	set bool
	v   E
}

func (o optional[E]) Or(v E) E {
	if o.set {
		return o.v
	}
	return v
}

type optionalC[E comparable] optional[E]

// Is returns whether the optional is validly set to v. Shorthand for
// o.set && o.v == v.
func (o optionalC[E]) Is(v E) bool {
	return o.set && o.v == v
}

func (o optionalC[E]) Or(v E) E {
	return (optional[E](o)).Or(v)
}

type projAction int

const (
	projInfo projAction = iota
	projGet
	projNew
	projEdit
)

var (
	flagProjNew bool
)

type projKey string

const (
	projKeyName           projKey = "NAME"
	projKeyHistFile       projKey = "HISTORY-FILE"
	projKeySeshFile       projKey = "SESSION-FILE"
	projKeyCookieLifetime projKey = "COOKIE-LIFETIME"
	projKeyCookies        projKey = "COOKIES"
	projKeyHistory        projKey = "HISTORY"
)

// Human prints the human-readable description of the key.
func (pk projKey) Human() string {
	switch pk {
	case projKeyName:
		return "project name"
	case projKeyHistFile:
		return "history file"
	case projKeySeshFile:
		return "session file"
	case projKeyCookieLifetime:
		return "cookie lifetime"
	case projKeyCookies:
		return "cookie recording"
	case projKeyHistory:
		return "history recording"
	default:
		return fmt.Sprintf("unknown project key %q", pk)
	}
}

func (pk projKey) Name() string {
	return string(pk)
}

// extracts map keys in order of projAttrKeys
func sortedProjAttrMapKeys[E any](m map[projKey]E) []projKey {
	keys := []projKey{}
	for _, k := range projAttrKeys {
		if _, ok := m[k]; ok {
			keys = append(keys, k)
		}
	}
	return keys
}

var (
	// ordering of projAttrKeys in output is set here

	projAttrKeys = []projKey{
		projKeyName,
		projKeyHistFile,
		projKeyHistory,
		projKeySeshFile,
		projKeyCookies,
		projKeyCookieLifetime,
	}
)

func projAttrKeyNames() []string {
	names := make([]string, len(projAttrKeys))
	for i, k := range projAttrKeys {
		names[i] = k.Name()
	}
	return names
}

func parseProjAttrKey(s string) (projKey, error) {
	switch strings.ToUpper(s) {
	case projKeyName.Name():
		return projKeyName, nil
	case projKeyHistFile.Name():
		return projKeyHistFile, nil
	case projKeySeshFile.Name():
		return projKeySeshFile, nil
	case projKeyCookieLifetime.Name():
		return projKeyCookieLifetime, nil
	case projKeyCookies.Name():
		return projKeyCookies, nil
	case projKeyHistory.Name():
		return projKeyHistory, nil
	default:
		return "", fmt.Errorf("invalid attribute %q; must be one of %s", s, strings.Join(projAttrKeyNames(), ", "))
	}
}

func parseOnOff(s string) (bool, error) {
	up := strings.ToUpper(s)
	if up == "ON" || up == "1" || up == "ENABLE" || up == "TRUE" || up == "T" || up == "YES" || up == "Y" {
		return true, nil
	}

	if up == "OFF" || up == "0" || up == "DISABLE" || up == "FALSE" || up == "F" || up == "NO" || up == "N" {
		return false, nil
	}

	return false, fmt.Errorf("invalid value %q; must be ON or OFF (case-insensitive)", s)
}

type projOptions struct {
	projFile string
	action   projAction

	name           optionalC[string] // only used directly in edit; new takes new name directly
	recordHistory  optionalC[bool]
	recordCookies  optionalC[bool]
	seshFile       optionalC[string]
	histFile       optionalC[string]
	cookieLifetime optionalC[time.Duration]
}

func (eo projOptions) changesFilePaths() bool {
	return eo.seshFile.set || eo.histFile.set
}

func invokeProjEdit(io cmdio.IO, opts projOptions) error {
	// if either the history file or session file are altered, or if cookie
	// lifetime is altered, we need to load the current files to mutate or
	// copy the data
	modifyAllFiles := opts.changesFilePaths() || opts.cookieLifetime.set

	// load the project file
	p, err := morc.LoadProjectFromDisk(opts.projFile, modifyAllFiles)
	if err != nil {
		return err
	}

	modifiedVals := map[projKey]interface{}{}
	noChangeVals := map[projKey]interface{}{}

	if opts.name.set {
		if opts.name.v == p.Name {
			noChangeVals[projKeyName] = p.Name
		} else {
			p.Name = opts.name.v
			modifiedVals[projKeyName] = p.Name
		}
	}

	if opts.histFile.set {
		if opts.histFile.v == "" {
			// set to empty string effectively disables history, so we cannot do
			// this if history is enabled or if it is about to be.

			// history is on and not turning it off
			if p.Config.RecordHistory && !opts.recordHistory.Is(false) {
				return fmt.Errorf("cannot set history file to empty string: history recording must be disabled first")
			}

			// history is off and turning it on
			if !p.Config.RecordHistory && opts.recordHistory.Is(true) {
				return fmt.Errorf("cannot set history file to empty string when passing --hist-on")
			}

			// otherwise, it is safe to set it.
		}

		if opts.histFile.v == p.Config.HistFile {
			noChangeVals[projKeyHistFile] = p.Config.HistFile
		} else {
			p.Config.HistFile = opts.histFile.v
			modifiedVals[projKeyHistFile] = p.Config.HistFile
		}
	}

	if opts.seshFile.set {
		if opts.seshFile.v == "" {
			// set to empty string effectively disables session saving, so we
			// cannot do this if cookie saving is enabled or if it is about to
			// be.

			// cookies are on and not turning it off
			if p.Config.RecordSession && !opts.recordCookies.Is(false) {
				return fmt.Errorf("cannot set session file to empty string: cookie recording must be disabled first")
			}

			// cookies are off and turning it on
			if !p.Config.RecordSession && opts.recordCookies.Is(true) {
				return fmt.Errorf("cannot set session file to empty string when passing --cookies-on")
			}

			// otherwise, it is safe to set it.
		}

		if opts.seshFile.v == p.Config.SeshFile {
			noChangeVals[projKeySeshFile] = p.Config.SeshFile
		} else {
			p.Config.SeshFile = opts.seshFile.v
			modifiedVals[projKeySeshFile] = p.Config.SeshFile
		}
	}

	if opts.cookieLifetime.set {
		if opts.cookieLifetime.v == p.Config.CookieLifetime {
			noChangeVals[projKeyCookieLifetime] = p.Config.CookieLifetime
		} else {
			p.Config.CookieLifetime = opts.cookieLifetime.v
			p.EvictOldCookies()
			modifiedVals[projKeyCookieLifetime] = p.Config.CookieLifetime
		}
	}

	if opts.recordHistory.set {
		// enabling is not allowed if the history file is unset
		if p.Config.HistFile == "" && opts.recordHistory.Is(true) {
			return fmt.Errorf("cannot enable history recording: no history file set")
		}

		if opts.recordHistory.v == p.Config.RecordHistory {
			noChangeVals[projKeyHistory] = p.Config.RecordHistory
		} else {
			p.Config.RecordHistory = opts.recordHistory.v
			modifiedVals[projKeyHistory] = p.Config.RecordHistory
		}
	}

	if opts.recordCookies.set {
		// enabling is not allowed if the session file is unset
		if p.Config.SeshFile == "" && opts.recordCookies.Is(true) {
			return fmt.Errorf("cannot enable cookie recording: no session file set")
		}

		if opts.recordCookies.v == p.Config.RecordSession {
			noChangeVals[projKeyCookies] = p.Config.RecordSession
		} else {
			p.Config.RecordSession = opts.recordCookies.v
			modifiedVals[projKeyCookies] = p.Config.RecordSession
		}
	}

	err = p.PersistToDisk(modifyAllFiles)
	if err != nil {
		return err
	}

	// create our output
	if len(modifiedVals) > 0 {
		io.PrintLoudf("Set ")

		// get ordering we want
		modKeys := sortedProjAttrMapKeys(modifiedVals)

		// turn to slice of output values and let IO handle commas
		setMessages := []string{}
		for _, k := range modKeys {
			v := modifiedVals[projKey(k)]

			if fmt.Sprintf("%v", v) == "" {
				v = `""`
			}

			setMessages = append(setMessages, fmt.Sprintf("%s to %s", k.Human(), v))
		}

		io.PrintLoudf("%s\n", io.OxfordCommaJoin(setMessages))
	}

	if len(noChangeVals) > 0 {
		// get ordering we want
		noChangeKeys := sortedProjAttrMapKeys(noChangeVals)

		// we don't need to do fancy string building because we will simply output
		// each one on its own line
		for _, k := range noChangeKeys {
			v := noChangeVals[projKey(k)]

			if fmt.Sprintf("%v", v) == "" {
				v = `""`
			}

			io.PrintLoudErrf("No change to %s; already set to %s\n", k.Human(), v)
		}
	}

	return nil
}

func invokeProjNew(io cmdio.IO, name string, opts projOptions) error {
	// make sure the user isn't about to turn on history without setting a file
	if opts.recordHistory.set && opts.recordHistory.v && opts.histFile.v == "" {
		return fmt.Errorf("cannot create project with history enabled without setting a history file")
	}

	// make sure the user isn't about to turn on cookies without setting a file
	if opts.recordCookies.set && opts.recordCookies.v && opts.seshFile.v == "" {
		return fmt.Errorf("cannot create project with cookie recording enabled without setting a session file")
	}

	// okay we are good, proceed to create the project

	p := morc.Project{
		Name:      name,
		Templates: map[string]morc.RequestTemplate{},
		Flows:     map[string]morc.Flow{},
		Vars:      morc.NewVarStore(),
		History:   []morc.HistoryEntry{},
		Session:   morc.Session{},
		Config: morc.Settings{
			ProjFile:       opts.projFile,
			HistFile:       opts.histFile.v,
			SeshFile:       opts.seshFile.v,
			CookieLifetime: opts.cookieLifetime.Or(24 * time.Hour),
			RecordSession:  opts.recordCookies.v,
			RecordHistory:  opts.recordHistory.v,
		},
	}

	err := p.PersistToDisk(false)
	if err != nil {
		return err
	}

	if opts.histFile.v != "" {
		// persist at least once so user knows right away if it is a bad path

		err = p.PersistHistoryToDisk()
		if err != nil {
			return err
		}
	}

	if opts.seshFile.v != "" {
		// persist at least once so user knows right away if it is a bad path

		err = p.PersistSessionToDisk()
		if err != nil {
			return err
		}
	}

	io.PrintLoudf("Project created successfully in %s\n", opts.projFile)

	return nil
}

func invokeProjGet(io cmdio.IO, item projKey, opts projOptions) error {
	proj, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	switch item {
	case projKeyName:
		io.Printf("%s\n", proj.Name)
	case projKeyHistFile:
		io.Printf("%s\n", proj.Config.HistFile)
	case projKeySeshFile:
		io.Printf("%s\n", proj.Config.SeshFile)
	case projKeyCookieLifetime:
		io.Printf("%s\n", proj.Config.CookieLifetime)
	case projKeyCookies:
		io.Printf("%s\n", io.OnOrOff(proj.Config.RecordSession))
	case projKeyHistory:
		io.Printf("%s\n", io.OnOrOff(proj.Config.RecordHistory))
	default:
		panic(fmt.Sprintf("unhandled proj key %q", item))
	}

	return nil
}

func invokeProjShow(io cmdio.IO, opts projOptions) error {
	proj, err := morc.LoadProjectFromDisk(opts.projFile, true)
	if err != nil {
		return err
	}

	io.Printf("Project: %s\n", proj.Name)
	io.Printf("%s, %s\n", io.CountOf(len(proj.Templates), "request"), io.CountOf(len(proj.Flows), "flow"))
	io.Printf("%s\n", io.CountOf(len(proj.History), "history item"))
	io.Printf("%s across %s\n", io.CountOf(proj.Vars.Count(), "variable"), io.CountOf(proj.Vars.EnvCount(), "environment"))
	io.Printf("%s in active session\n", io.CountOf(proj.Session.TotalCookieSets(), "cookie"))
	io.Println()
	io.Printf("Cookie record lifetime: %s\n", proj.Config.CookieLifetime)
	io.Printf("Project file on record: %s\n", proj.Config.ProjFile)
	io.Printf("Session file on record: %s\n", proj.Config.SeshFile)
	io.Printf("History file on record: %s\n", proj.Config.HistFile)
	io.Printf("Cookie recording is %s\n", io.OnOrOff(proj.Config.RecordSession))
	io.Printf("History tracking is %s\n", io.OnOrOff(proj.Config.RecordHistory))
	io.Println()
	if proj.Vars.Environment == "" {
		io.Printf("Using default var environment\n")
	} else {
		io.Printf("Using var environment %s\n", proj.Vars.Environment)
	}

	return nil
}
