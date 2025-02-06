package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

// INVOKES WE MUST SUPPORT:
// cli invoke: morc auths -N basic-login -t basic -u username -p password
// cli invoke: morc auths -N auth-name -t session -r retrievial-spec -c cookie --no-exp
// cli invoke: morc auths -N auth-name -t token -r retrieval-spec --token-from token-scraper --use-in header:key-name --use-format bearer --exp expires-scraper --exp-layout expires-layout
// cli invoke: morc auths -N auth-name -t jwt -r retrieval-spec --token-from token-scraper

var authsCmd = &cobra.Command{
	Use: "auths [AUTH]",
	Annotations: map[string]string{
		annotationKeyHelpUsages: "" +
			"auths\n" +
			"auths --delete AUTH [-f]\n" +
			"auths --new AUTH \n" + // TODO: actual auth parameters
			"auths AUTH\n" +
			"auths AUTH --get ATTR\n" +
			"auths AUTH \n" + // TODO: actual auth parameters.
			"auths --clear AUTH",
	},
	GroupID: "project",
	Short:   "Show or modify authorization methods",
	Long:    "", // TODO: fill help.
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		var args authsArgs
		if err := parseAuthsArgs(cmd, posArgs, &args); err != nil {
			return err
		}

		// done checking args, don't show usage on error
		cmd.SilenceUsage = true
		io := cmdio.From(cmd)
		io.Quiet = flags.BQuiet

		switch args.action {
		default:
			panic(fmt.Sprintf("unhandled auths action %q", args.action))
		}
	},
}

func init() {

	// TODO: make these all non-required.
	// TODO: add concept of "useable" auth that has its properties set.

	// cli invoke: morc auths -N basic-login -t basic -u username -p password
	// cli invoke: morc auths -N auth-name -t session -r retrievial-spec -c cookie --no-exp
	// cli invoke: morc auths -N auth-name -t token -r retrieval-spec --token-from token-scraper --use-in header:key-name --use-format bearer --exp-from expires-scraper --exp-layout expires-layout
	// cli invoke: morc auths -N auth-name -t jwt -r retrieval-spec --token-from token-scraper
	authsCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	authsCmd.PersistentFlags().StringVarP(&flags.New, "new", "N", "", "Create a new auth method named `AUTH`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Delete, "delete", "D", "", "Delete the auth method named `AUTH`.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BForce, "force", "f", false, "Force deletion of an auth method even if it is used in a request.")
	authsCmd.PersistentFlags().StringVarP(&flags.Get, "get", "G", "", "Get the value of the given attribute `ATTR` from the auth method. ATTR must be one of: "+strings.Join(authAttrKeyNames(), ", "))
	authsCmd.PersistentFlags().StringVarP(&flags.Clear, "clear", "C", "", "Clear any currently saved auth proof. Only applicable to auth types that use dynamically-retrieved proofs, such a cookie or a token.")
	authsCmd.PersistentFlags().StringVarP(&flags.Name, "name", "n", "", "Change the name of an auth method to `NAME`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Type, "type", "t", "", "Set the type of auth method to `TYPE`. TYPE must be one of 'basic', 'session', 'jwt', or 'token'; the choice determined what other options are available.")
	authsCmd.PersistentFlags().StringVarP(&flags.Username, "username", "u", "", "Set the `USERNAME` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Password, "password", "p", "", "Set the `PASSWORD` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Retrieval, "retrieval", "r", "", "Set the flow or request template to use to retrieve proof of authentication. This is a string of the form F:NAME for a flow or R:NAME for a request template; if no prefix is given, it is assumed to be a flow name. Only valid when --type is 'session', 'jwt', or 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Cookie, "cookie", "c", "", "Set the name of the cookie to get session information from to `NAME`. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BNoExpiration, "no-exp", "", false, "Do not detect an expiration time for the auth proof, resulting in it being used until an invalid auth is detected. Only valid when --type is 'session' or 'token'.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BYesExpiration, "exp", "", false, "Enable detection of an expiration time for a session cookie based auth proof, resulting in a new one being automatically retrieved before the authenticated request if the currently held one has expired. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().StringVarP(&flags.TokenScraper, "token-scraper", "T", "", "Set the scraper to use to extract the token from the last response of auth proof retrieval. Only valid when --type is 'jwt' or 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Dest, "dest", "d", "", "Set where the token should be used in the authenticated request. `LOCATION` must be either 'header:NAME-OF-HEADER' or 'cookie:NAME-OF-COOKIE'. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Format, "format", "", "", "Set the format of the token proof in the authenticated to `FORMAT`. If not set, the token's exact value is used. If set to `bearer`, it's value will be preceded by the word 'Bearer'. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.ExpirationScraper, "exp-scraper", "X", "", "Set the scraper to use to extract the expiration time of the token from the last response of auth proof retrieval. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.ExpirationLayout, "exp-layout", "L", "RFC3339", "Set the layout of the expiration time of the token to `LAYOUT`. This can either be a custom string that is Go time layout format, or one of the following constants: 'RFC822', 'RFC822Z', 'RFC850', 'RFC1123', 'RFC1123Z', 'RFC3339', or 'RFC3339Nano'. Only valid when --type is 'token'.")

	reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "clear")

	// don't specify attribute args if not creating or setting.
	reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "clear", "name")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "type")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "username")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "password")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "retrieval")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "cookie")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "no-exp")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "exp")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "token-scraper")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "dest")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "format")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "exp-scraper")
	reqsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "exp-layout")

	reqsCmd.MarkFlagsMutuallyExclusive("new", "get", "clear", "force")
	reqsCmd.MarkFlagsMutuallyExclusive("no-exp", "exp-scraper")
	reqsCmd.MarkFlagsMutuallyExclusive("no-exp", "exp-layout")

	reqsCmd.MarkFlagsMutuallyExclusive("no-exp", "exp")

	rootCmd.AddCommand(authsCmd)
}

type authsArgs struct {
	projFile string
	action   authsAction
	getItem  authKey
	force    bool
	auth     string

	sets authAttrValues
}

type authAttrValues struct {
	name     optional[string]
	authType optional[string]

	username optional[string]
	password optional[string]

	retrieval           optional[morc.RequestSequence]
	expirationDetection optional[bool]

	cookie optional[string]

	tokenSpec optional[morc.Scraper]
	dest      optional[morc.ProofDestination]
	format    optional[morc.ProofFormat]

	expirationSpec   optional[morc.Scraper]
	expirationLayout optional[string]
}

func parseAuthsArgs(cmd *cobra.Command, posArgs []string, args *authsArgs) error {
	args.projFile = projPathFromFlagsOrFile(cmd)
	if args.projFile == "" {
		return fmt.Errorf("project file cannot be set to empty string")
	}

	var err error

	args.action, err = parseAuthsActionFromFlags(cmd, posArgs)
	if err != nil {
		return err
	}

	// do action-specific arg and flag parsing
	switch args.action {
	case authsActionList:
		// nothing else to do
	case authsActionShow:
		// use arg 1 as the auth name
		args.auth = posArgs[0]
	case authsActionClear:
		// special case of auth name set from a CLI flag rather than pos arg.
		args.auth = flags.Clear
	case authsActionDelete:
		// special case of auth name set from a CLI flag rather than pos arg.
		args.auth = flags.Delete

		args.force = flags.BForce
	case authsActionGet:
		// use arg 1 as the auth name
		args.auth = posArgs[0]

		args.getItem, err = parseAuthAttrKey(flags.Get)
		if err != nil {
			return err
		}
	case authsActionNew:
		// above action parsing already checked that invalid set opts will not
		// be present so we can just call parseAuthsSetFlags and then use
		// --new argument to set the new auth method name.
		if err := parseAuthsSetFlags(cmd, &args.sets); err != nil {
			return err
		}

		// set auth name from the flag
		args.auth = flags.New
		args.sets.name = optional[string]{set: true, v: flags.New}
	case authsActionEdit:
		// use arg 1 as the auth name
		args.auth = posArgs[0]

		if err := parseAuthsSetFlags(cmd, &args.sets); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unhandled auths action %q", args.action))
	}

	return nil
}

func parseAuthsActionFromFlags(cmd *cobra.Command, posArgs []string) (authsAction, error) {
	// mutual exclusions enforced by cobra (and therefore we do not check them here):
	// * --new, --delete, --get, and --clear
	// * --delete with mod flags
	// * --get with mod flags
	// * --clear with mod flags
	// * --force with --get, --clear, and --new
	// * --no-exp with any flag that indicates expiration detection

	// make sure user isn't invalidly using -f because cobra is not enforcing this
	if flags.BForce && flags.Delete == "" {
		return authsAction(0), fmt.Errorf("--force/-f can only be used with --delete/-D")
	}

	if flags.Delete != "" {
		if len(posArgs) > 0 {
			return authsActionDelete, fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return authsActionDelete, nil
	} else if flags.New != "" {
		if len(posArgs) > 0 {
			return authsActionNew, fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return authsActionNew, nil
	} else if flags.Get != "" {
		if len(posArgs) < 1 {
			return authsActionGet, fmt.Errorf("missing name of AUTH to get from")
		}
		if len(posArgs) > 1 {
			return authsActionGet, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}
		return authsActionGet, nil
	} else if flags.Clear != "" {
		if len(posArgs) > 0 {
			return authsActionClear, fmt.Errorf("unknown positional argument %q", posArgs[0])
		}
		return authsActionClear, nil
	} else if authsSetFlagIsPresent(cmd) {
		if len(posArgs) < 1 {
			return authsActionEdit, fmt.Errorf("missing name of AUTH to update")
		}
		if len(posArgs) > 1 {
			return authsActionEdit, fmt.Errorf("unknown positional argument %q", posArgs[1])
		}
		return authsActionEdit, nil
	}

	if len(posArgs) == 0 {
		return authsActionList, nil
	} else if len(posArgs) == 1 {
		return authsActionShow, nil
	} else {
		return authsAction(0), fmt.Errorf("unknown positional argument %q", posArgs[1])
	}
}

func parseAuthsSetFlags(cmd *cobra.Command, attrs *authAttrValues) error {
	f := cmd.Flags()

	if f.Changed("name") {
		attrs.name = optional[string]{set: true, v: flags.Name}
	}

	if f.Changed("type") {
		tLower := strings.ToLower(flags.Type)
		if tLower != "basic" && tLower != "session" && tLower != "jwt" && tLower != "token" {
			return fmt.Errorf("invalid auth type %q; must be one of 'basic', 'session', 'jwt', or 'token'", flags.Type)
		}
		attrs.authType = optional[string]{set: true, v: flags.Type}
	}

	if f.Changed("username") {
		attrs.username = optional[string]{set: true, v: flags.Username}
	}

	if f.Changed("password") {
		attrs.password = optional[string]{set: true, v: flags.Password}
	}

	if f.Changed("retrieval") {
		seq, err := morc.ParseRequestSequence(flags.Retrieval)
		if err != nil {
			return fmt.Errorf("--retrieval/-r: %w", err)
		}
		attrs.retrieval = optional[morc.RequestSequence]{set: true, v: seq}
	}

	if f.Changed("cookie") {
		attrs.cookie = optional[string]{set: true, v: flags.Cookie}
	}

	if f.Changed("no-exp") {
		attrs.expirationDetection = optional[bool]{set: true, v: false}
	}

	if f.Changed("exp") {
		attrs.expirationDetection = optional[bool]{set: true, v: true}
	}

	if f.Changed("token-scraper") {
		scraper, err := morc.ParseVarScraperSpec("token", flags.TokenScraper)
		if err != nil {
			return fmt.Errorf("--token-scraper/-T: %w", err)
		}
		attrs.tokenSpec = optional[morc.Scraper]{set: true, v: scraper}
	}

	if f.Changed("dest") {
		parts := strings.SplitN(flags.Dest, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("--dest/-d: must be in the form 'header:NAME' or 'cookie:NAME'")
		}
		loc, err := morc.ParseProofLocation(parts[0])
		if err != nil {
			return fmt.Errorf("--dest/-d: %q: %w", parts[0], err)
		}
		key := parts[1]
		if key == "" {
			return fmt.Errorf("--dest/-d: key name must not be empty")
		}

		pd := morc.ProofDestination{
			Location: loc,
			Key:      key,
		}

		attrs.dest = optional[morc.ProofDestination]{set: true, v: pd}
	}

	if f.Changed("format") {
		format, err := morc.ParseProofFormat(flags.Format)
		if err != nil {
			return fmt.Errorf("--format: %w", err)
		}

		attrs.format = optional[morc.ProofFormat]{set: true, v: format}
	}

	if f.Changed("exp-scraper") {
		scraper, err := morc.ParseVarScraperSpec("expiration", flags.ExpirationScraper)
		if err != nil {
			return fmt.Errorf("--exp-scraper/-X: %w", err)
		}
		attrs.expirationSpec = optional[morc.Scraper]{set: true, v: scraper}
	}

	if f.Changed("exp-layout") {
		if flags.ExpirationLayout == "" {
			return fmt.Errorf("--exp-layout/-L: layout must not be empty")
		}

		// parse for the special go time layout constants
		layout := flags.ExpirationLayout
		layoutUpper := strings.ToUpper(layout)

		if layoutUpper == "RFC822" {
			layout = time.RFC822
		} else if layoutUpper == "RFC822Z" {
			layout = time.RFC822Z
		} else if layoutUpper == "RFC850" {
			layout = time.RFC850
		} else if layoutUpper == "RFC1123" {
			layout = time.RFC1123
		} else if layoutUpper == "RFC1123Z" {
			layout = time.RFC1123Z
		} else if layoutUpper == "RFC3339" {
			layout = time.RFC3339
		} else if layoutUpper == "RFC3339NANO" {
			layout = time.RFC3339Nano
		}

		attrs.expirationLayout = optional[string]{set: true, v: layout}
	}

	return nil
}

func authsSetFlagIsPresent(cmd *cobra.Command) bool {
	f := cmd.Flags()
	return f.Changed("name") ||
		f.Changed("type") ||
		f.Changed("username") ||
		f.Changed("password") ||
		f.Changed("retrieval") ||
		f.Changed("no-exp") ||
		f.Changed("exp") ||
		f.Changed("token-scraper") ||
		f.Changed("exp-scraper") ||
		f.Changed("exp-layout") ||
		f.Changed("cookie") ||
		f.Changed("dest") ||
		f.Changed("format")
}

type authsAction int

const (
	authsActionList authsAction = iota
	authsActionShow
	authsActionNew
	authsActionDelete
	authsActionGet
	authsActionEdit
	authsActionClear
)

type authKey string

// TODO: make args match these names, i like them betta glub.
const (
	authKeyName         authKey = "NAME"
	authKeyType         authKey = "TYPE"
	authKeyUsername     authKey = "USERNAME"
	authKeyPassword     authKey = "PASSWORD"
	authKeyCookie       authKey = "COOKIE"
	authKeyExpDetection authKey = "EXP-DETECTION"
	authKeyRetrieval    authKey = "RETRIEVAL"
	authKeyTokenScraper authKey = "TOKEN-SCRAPER"
	authKeyDest         authKey = "DEST"
	authKeyFormat       authKey = "FORMAT"
	authKeyExpScraper   authKey = "EXP-SCRAPER"
	authKeyExpLayout    authKey = "EXP-LAYOUT"
)

func (ak authKey) Human() string {
	switch ak {
	case authKeyName:
		return "auth method name"
	case authKeyType:
		return "auth method type"
	case authKeyUsername:
		return "basic auth username"
	case authKeyPassword:
		return "basic auth password"
	case authKeyCookie:
		return "session cookie"
	case authKeyExpDetection:
		return "auth proof expiration detection"
	case authKeyRetrieval:
		return "auth proof retrieval sequence"
	case authKeyTokenScraper:
		return "token scraper spec"
	case authKeyDest:
		return "auth proof destination"
	case authKeyFormat:
		return "auth proof value format"
	case authKeyExpScraper:
		return "auth proof expiration scraper"
	case authKeyExpLayout:
		return "auth proof expiration layout"
	default:
		return string(ak)
	}
}

func (ak authKey) Name() string {
	return string(ak)
}

var (
	// ordering of authAttrKeys in output is set here

	authAttrKeys = []authKey{
		authKeyName,
		authKeyType,
		authKeyRetrieval,
		authKeyUsername,
		authKeyPassword,
		authKeyCookie,
		authKeyExpDetection,
		authKeyDest,
		authKeyFormat,
		authKeyTokenScraper,
		authKeyExpScraper,
		authKeyExpLayout,
	}
)

func authAttrKeyNames() []string {
	names := make([]string, len(authAttrKeys))
	for i, k := range authAttrKeys {
		names[i] = k.Name()
	}
	return names
}

func parseAuthAttrKey(s string) (authKey, error) {
	sUpper := strings.ToUpper(s)
	for _, k := range authAttrKeys {
		if k.Name() == sUpper {
			return k, nil
		}
	}
	return "", fmt.Errorf("invalid attribute %q; must be one of %s", s, strings.Join(authAttrKeyNames(), ", "))
}
