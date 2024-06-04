package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/dekarrin/morc/cmd/morc/commonflags"
	"github.com/dekarrin/rosed"
	"github.com/spf13/cobra"
)

var (
	flagProjNew            bool
	flagProjGet            string
	flagProjName           string
	flagProjHistoryFile    string
	flagProjSessionFile    string
	flagProjCookieLifetime string
	flagProjRecordCookies  string
	flagProjRecordHistory  string
)

var projCmd = &cobra.Command{
	Use: "proj",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"proj\n" +
			"proj --new [-nHSCcR]\n" +
			"proj --get ATTR\n" +
			"proj [-nHSCcR]",
	},
	GroupID: "project",
	Short:   "Show or manipulate project attributes and config",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var opts projArgs
		if err := parseProjArgs(cmd, args, &opts); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)

		switch opts.action {
		case projInfo:
			return invokeProjShow(io, opts.projFile)
		case projGet:
			return invokeProjGet(io, opts.projFile, opts.getItem)
		case projNew:
			return invokeProjNew(io, opts.projFile, opts.sets)
		case projEdit:
			return invokeProjEdit(io, opts.projFile, opts.sets)
		default:
			panic(fmt.Sprintf("unhandled proj action %q", opts.action))
		}
	},
}

func init() {
	projCmd.PersistentFlags().StringVarP(&commonflags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	projCmd.PersistentFlags().BoolVarP(&flagProjNew, "new", "N", false, "Create a new project instead of reading/editing one. Combine with other arguments to specify values for the new project.")
	projCmd.PersistentFlags().StringVarP(&flagProjGet, "get", "G", "", "Get the value of a specific attribute of the project. `ATTR` is the name of an attribute to retrieve and must be one of the following: "+strings.Join(projAttrKeyNames(), ", "))
	projCmd.PersistentFlags().StringVarP(&flagProjName, "name", "n", "", "Set the name of the project to `NAME`")
	projCmd.PersistentFlags().StringVarP(&flagProjHistoryFile, "history-file", "H", "", "Set the history file to `FILE`. If the special string '"+morc.ProjDirVar+"' is in the path, it is replaced with the directory containing the project file whenever morc is executed, allowing the history file path to still function even if the containing directory is moved.")
	projCmd.PersistentFlags().StringVarP(&flagProjSessionFile, "cookies-file", "C", "", "Set the session (cookies) storage file to `FILE`. If the special string '"+morc.ProjDirVar+"' is in the path, it is replaced with the directory containing the project file whenever morc is executed, allowing the session file path to still function even if the containing directory is moved.")
	projCmd.PersistentFlags().StringVarP(&flagProjCookieLifetime, "cookie-lifetime", "L", "", "Set the lifetime of recorded cookies to `DUR`. DUR must be a duration string such as 8m2s or similar. If set to 0 or less, it will be interpreted as '24h'. Altering this on an existing project will immediately apply an eviction check to all current cookies; this may result in some being purged.")
	projCmd.PersistentFlags().StringVarP(&flagProjRecordCookies, "cookies", "c", "", "Set whether cookie recording is enabled. `ON|OFF` must be one of 'ON' or 'OFF'. Setting this is equivalent to calling 'morc cookies --on' or 'morc cookies --off'")
	projCmd.PersistentFlags().StringVarP(&flagProjRecordHistory, "history", "R", "", "Set whether history recording is enabled. `ON|OFF` must be one of 'ON' or 'OFF'. Setting this is equivalent to calling 'morc history --on' or 'morc history --off'")

	projCmd.MarkFlagsMutuallyExclusive("new", "get")
	projCmd.MarkFlagsMutuallyExclusive("cookies", "get")
	projCmd.MarkFlagsMutuallyExclusive("cookie-lifetime", "get")
	projCmd.MarkFlagsMutuallyExclusive("history", "get")
	projCmd.MarkFlagsMutuallyExclusive("history-file", "get")
	projCmd.MarkFlagsMutuallyExclusive("cookies-file", "get")

	customFormattedCommandDescriptions[projCmd.Name()] = longHelp{fn: projCmdHelp, resultIsWrapped: true}

	rootCmd.AddCommand(projCmd)
}

var (
	projCmdHelp = func() string {
		s := "Shows and manipulates MORC project files. By default, this will "
		s += "operate on the project file located in .morc/project.json relative "
		s += "to the current working directory. A different file can be specified "
		s += "with the --project-file/-F flag.\n"
		s += "\n"
		s += "A new project can be created by passing --new, along with any number of flags "
		s += "to specify values for attributes of the new project.\n"
		s += "\n"
		s += "If no arguments are given, a summary of the project is printed. Specific "
		s += "attributes can be retrieved by passing --get along with the name of an attribute. "
		s += "The attribute name must be one of the those listed in the attributes section.\n"
		s += "\n"
		s += "The project can be modified by passing any flag allowed with --new "
		s += "and provided a value.\n"
		s += "\n"
		s += "Attributes for --get:\n"

		// above is starting string, load it into a roseditor and then insert
		// the attributes as definitions list
		attributes := [][2]string{
			{projKeyName.Name(), "The name of the project."},
			{projKeyHistFile.Name(), "The path to the history file. Does not affect whether request history is actually recorded; see " + projKeyHistory.Name() + " for that. If the special string '" + morc.ProjDirVar + "' is in the path, it is replaced with the directory containing the project file whenever morc is executed, allowing the history file path to still function even if the containing directory is moved."},
			{projKeyHistory.Name(), "Whether history recording is enabled. The value will either be the string 'ON' or 'OFF' (case-insensitive). Setting this is equivalent to calling 'morc history --on' or 'morc history --off'"},
			{projKeySeshFile.Name(), "The path to the session file. Does not affect whether sessions (cookies) are actually recorded; use " + projKeyCookies.Name() + " for that. If the special string '" + morc.ProjDirVar + "' is in the path, it is replaced with the directory containing the project file whenever morc is executed, allowing the session file path to still function even if the containing directory is moved."},
			{projKeyHistory.Name(), "Whether cookie recording is enabled. When setting, the value must must be the string 'ON' or 'OFF' (case-insensitive). Setting this is equivalent to calling 'morc cookies --on' or 'morc cookies --off'"},
			{projKeyCookieLifetime.Name(), "The lifetime of recorded Set-Cookie calls. When setting, the value must be a duration such as '24h' or '1h30m'. If set to 0 or less, it will be interpreted as 24h. Altering this will immediately apply an eviction check to all current cookies; this may result in some being purged."},
		}

		// format all with roseditor.
		width := getWrapWidth()
		return rosed.
			Edit(s).
			WithOptions(rosed.Options{
				PreserveParagraphs: true,
			}).
			Wrap(width).
			Insert(rosed.End, "\n"). // above wrap clobbers newline for some reason
			InsertDefinitionsTable(rosed.End, attributes, width).
			String()
	}
)

func invokeProjEdit(io cmdio.IO, projFile string, attrs projAttrValues) error {
	// if either the history file or session file are altered, or if cookie
	// lifetime is altered, we need to load the current files to mutate or
	// copy the data
	modifyAllFiles := attrs.changesFilePaths() || attrs.cookieLifetime.set

	// load the project file
	p, err := morc.LoadProjectFromDisk(projFile, modifyAllFiles)
	if err != nil {
		return err
	}

	modifiedVals := map[projKey]interface{}{}
	noChangeVals := map[projKey]interface{}{}

	if attrs.name.set {
		if attrs.name.v == p.Name {
			noChangeVals[projKeyName] = p.Name
		} else {
			p.Name = attrs.name.v
			modifiedVals[projKeyName] = p.Name
		}
	}

	if attrs.histFile.set {
		if attrs.histFile.v == "" {
			// set to empty string effectively disables history, so we cannot do
			// this if history is enabled or if it is about to be.

			// history is on and not turning it off
			if p.Config.RecordHistory && !attrs.recordHistory.Is(false) {
				return fmt.Errorf("cannot set history file to empty string: history recording must be disabled first")
			}

			// history is off and turning it on
			if !p.Config.RecordHistory && attrs.recordHistory.Is(true) {
				return fmt.Errorf("cannot set history file to empty string when passing --hist-on")
			}

			// otherwise, it is safe to set it.
		}

		if attrs.histFile.v == p.Config.HistFile {
			noChangeVals[projKeyHistFile] = p.Config.HistFile
		} else {
			p.Config.HistFile = attrs.histFile.v
			modifiedVals[projKeyHistFile] = p.Config.HistFile
		}
	}

	if attrs.seshFile.set {
		if attrs.seshFile.v == "" {
			// set to empty string effectively disables session saving, so we
			// cannot do this if cookie saving is enabled or if it is about to
			// be.

			// cookies are on and not turning it off
			if p.Config.RecordSession && !attrs.recordCookies.Is(false) {
				return fmt.Errorf("cannot set session file to empty string: cookie recording must be disabled first")
			}

			// cookies are off and turning it on
			if !p.Config.RecordSession && attrs.recordCookies.Is(true) {
				return fmt.Errorf("cannot set session file to empty string when passing --cookies-on")
			}

			// otherwise, it is safe to set it.
		}

		if attrs.seshFile.v == p.Config.SeshFile {
			noChangeVals[projKeySeshFile] = p.Config.SeshFile
		} else {
			p.Config.SeshFile = attrs.seshFile.v
			modifiedVals[projKeySeshFile] = p.Config.SeshFile
		}
	}

	if attrs.cookieLifetime.set {
		if attrs.cookieLifetime.v == p.Config.CookieLifetime {
			noChangeVals[projKeyCookieLifetime] = p.Config.CookieLifetime
		} else {
			p.Config.CookieLifetime = attrs.cookieLifetime.v
			p.EvictOldCookies()
			modifiedVals[projKeyCookieLifetime] = p.Config.CookieLifetime
		}
	}

	if attrs.recordHistory.set {
		// enabling is not allowed if the history file is unset
		if p.Config.HistFile == "" && attrs.recordHistory.Is(true) {
			return fmt.Errorf("cannot enable history recording: no history file set")
		}

		if attrs.recordHistory.v == p.Config.RecordHistory {
			noChangeVals[projKeyHistory] = p.Config.RecordHistory
		} else {
			p.Config.RecordHistory = attrs.recordHistory.v
			modifiedVals[projKeyHistory] = p.Config.RecordHistory
		}
	}

	if attrs.recordCookies.set {
		// enabling is not allowed if the session file is unset
		if p.Config.SeshFile == "" && attrs.recordCookies.Is(true) {
			return fmt.Errorf("cannot enable cookie recording: no session file set")
		}

		if attrs.recordCookies.v == p.Config.RecordSession {
			noChangeVals[projKeyCookies] = p.Config.RecordSession
		} else {
			p.Config.RecordSession = attrs.recordCookies.v
			modifiedVals[projKeyCookies] = p.Config.RecordSession
		}
	}

	err = p.PersistToDisk(modifyAllFiles)
	if err != nil {
		return err
	}

	cmdio.OutputLoudEditAttrsResult(io, modifiedVals, noChangeVals, projAttrKeys)

	return nil
}

func invokeProjNew(io cmdio.IO, projFile string, attrs projAttrValues) error {
	// make sure the user isn't about to turn on history without setting a file
	if attrs.recordHistory.set && attrs.recordHistory.v && attrs.histFile.v == "" {
		return fmt.Errorf("cannot create project with history enabled without setting a history file")
	}

	// make sure the user isn't about to turn on cookies without setting a file
	if attrs.recordCookies.set && attrs.recordCookies.v && attrs.seshFile.v == "" {
		return fmt.Errorf("cannot create project with cookie recording enabled without setting a session file")
	}

	// okay we are good, proceed to create the project

	p := morc.Project{
		Name:      attrs.name.v,
		Templates: map[string]morc.RequestTemplate{},
		Flows:     map[string]morc.Flow{},
		Vars:      morc.NewVarStore(),
		History:   []morc.HistoryEntry{},
		Session:   morc.Session{},
		Config: morc.Settings{
			ProjFile:       projFile,
			HistFile:       attrs.histFile.v,
			SeshFile:       attrs.seshFile.v,
			CookieLifetime: attrs.cookieLifetime.Or(24 * time.Hour),
			RecordSession:  attrs.recordCookies.v,
			RecordHistory:  attrs.recordHistory.v,
		},
	}

	err := p.PersistToDisk(false)
	if err != nil {
		return err
	}

	if attrs.histFile.v != "" {
		// persist at least once so user knows right away if it is a bad path

		err = p.PersistHistoryToDisk()
		if err != nil {
			return err
		}
	}

	if attrs.seshFile.v != "" {
		// persist at least once so user knows right away if it is a bad path

		err = p.PersistSessionToDisk()
		if err != nil {
			return err
		}
	}

	io.PrintLoudf("Project created successfully in %s\n", projFile)

	return nil
}

func invokeProjGet(io cmdio.IO, projFile string, item projKey) error {
	proj, err := morc.LoadProjectFromDisk(projFile, true)
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

func invokeProjShow(io cmdio.IO, projFile string) error {
	proj, err := morc.LoadProjectFromDisk(projFile, true)
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

type projArgs struct {
	projFile string
	action   projAction
	getItem  projKey
	sets     projAttrValues
}

type projAttrValues struct {
	name           optionalC[string]
	recordHistory  optionalC[bool]
	recordCookies  optionalC[bool]
	seshFile       optionalC[string]
	histFile       optionalC[string]
	cookieLifetime optionalC[time.Duration]
}

func (sfv projAttrValues) changesFilePaths() bool {
	return sfv.seshFile.set || sfv.histFile.set
}

func parseProjArgs(cmd *cobra.Command, _ []string, args *projArgs) error {
	args.projFile = commonflags.ProjectFile
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseProjActionFromFlags()
	if err != nil {
		return err
	}

	// do action-specific arg and flag parsing
	switch args.action {
	case projInfo:
		// no-op; no further checks to do
	case projGet:
		// parse the get from the string
		args.getItem, err = parseProjAttrKey(flagProjGet)
		if err != nil {
			return err
		}
	case projNew:
		if err := parseProjSetFlags(cmd, &args.sets); err != nil {
			return err
		}
	case projEdit:
		if err := parseProjSetFlags(cmd, &args.sets); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unhandled proj action %q", args.action))
	}

	return nil
}

func parseProjActionFromFlags() (projAction, error) {
	// Enforcements assumed:
	// * mutual-exclusion enforced by cobra: --new and --get will not both be
	// present.
	// * mutual-exclusion enforced by cobra: Iff --get present, set-flags will
	// not be present.
	// * No-args.

	if flagProjGet != "" {
		return projGet, nil
	} else if flagProjNew {
		return projNew, nil
	} else if projSetFlagIsPresent() {
		return projEdit, nil
	}
	return projInfo, nil
}

func parseProjSetFlags(cmd *cobra.Command, attrs *projAttrValues) error {
	if cmd.Flags().Lookup("name").Changed {
		attrs.name = optionalC[string]{set: true, v: flagProjName}
	}

	if cmd.Flags().Lookup("history-file").Changed {
		attrs.histFile = optionalC[string]{set: true, v: flagProjHistoryFile}
	}

	if cmd.Flags().Lookup("cookies-file").Changed {
		attrs.seshFile = optionalC[string]{set: true, v: flagProjSessionFile}
	}

	if cmd.Flags().Lookup("cookie-lifetime").Changed {
		cl, err := time.ParseDuration(flagProjCookieLifetime)
		if err != nil {
			return fmt.Errorf("cookie-lifetime: %w", err)
		}
		attrs.cookieLifetime = optionalC[time.Duration]{set: true, v: cl}
	}

	if cmd.Flags().Lookup("cookies").Changed {
		isOn, err := parseOnOff(flagProjRecordCookies)
		if err != nil {
			return fmt.Errorf("cookies: %w", err)
		}
		attrs.recordCookies = optionalC[bool]{set: true, v: isOn}
	}

	if cmd.Flags().Lookup("history").Changed {
		isOn, err := parseOnOff(flagProjRecordHistory)
		if err != nil {
			return fmt.Errorf("history: %w", err)
		}
		attrs.recordHistory = optionalC[bool]{set: true, v: isOn}
	}

	return nil
}

func projSetFlagIsPresent() bool {
	return flagProjName != "" || flagProjHistoryFile != "" || flagProjSessionFile != "" || flagProjCookieLifetime != "" || flagProjRecordCookies != "" || flagProjRecordHistory != ""
}

type projAction int

const (
	projInfo projAction = iota
	projGet
	projNew
	projEdit
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
