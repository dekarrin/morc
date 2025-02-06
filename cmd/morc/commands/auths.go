package commands

import (
	"fmt"
	"strings"

	"github.com/dekarrin/morc"
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
			"auths AUTH --clear",
	},
	GroupID: "project",
	Short:   "Show or modify authorization methods",
	Long:    "", // TODO: fill help.
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, posArgs []string) error {
		return nil
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
	authsCmd.PersistentFlags().BoolVarP(&flags.BClear, "clear", "C", false, "Clear any currently saved auth proof. Only applicable to auth types that use dynamically-retrieved proofs, such a cookie or a token.")
	authsCmd.PersistentFlags().StringVarP(&flags.Name, "name", "n", "", "Change the name of an auth method to `NAME`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Type, "type", "t", "", "Set the type of auth method to `TYPE`. TYPE must be one of 'basic', 'session', 'jwt', or 'token'; the choice determined what other options are available.")
	authsCmd.PersistentFlags().StringVarP(&flags.Username, "username", "u", "", "Set the `USERNAME` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Password, "password", "p", "", "Set the `PASSWORD` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Cookie, "cookie", "c", "", "Set the name of the cookie to get session information from to `NAME`. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BNoExpiration, "no-exp", "", false, "Do not detect an expiration time for the session cookie, resulting in it being used until an invalid auth is detected. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Retrieval, "retrieval", "r", "", "Set the flow or request template to use to retrieve proof of authentication. This is a string of the form F:NAME for a flow or R:NAME for a request template; if no prefix is given, it is assumed to be a flow name. Only valid when --type is 'session', 'jwt', or 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.TokenScraper, "token-scraper", "", "", "Set the scraper to use to extract the token from the last response of auth proof retrieval. Only valid when --type is 'jwt' or 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Dest, "use-in", "", "", "Set where the token should be used in the authenticated request. `LOCATION` must be either 'header:NAME-OF-HEADER' or 'cookie:NAME-OF-COOKIE'. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Format, "format", "", "", "Set the format of the token proof in the authenticated to `FORMAT`. If not set, the token's exact value is used. If set to `bearer`, it's value will be preceded by the word 'Bearer'. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.ExpirationScraper, "exp-scraper", "", "", "Set the scraper to use to extract the expiration time of the token from the last response of auth proof retrieval. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.ExpirationLayout, "exp-layout", "", "RFC3339", "Set the layout of the expiration time of the token to `LAYOUT`. This can either be a custom string that is Go time layout format, or one of the following constants: 'RFC822', 'RFC822Z', 'RFC850', 'RFC1123', 'RFC1123Z', 'RFC3339', or 'RFC3339Nano'. Only valid when --type is 'token'.")

	reqsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "clear")

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
