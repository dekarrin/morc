package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dekarrin/morc"
	"github.com/dekarrin/morc/cmd/morc/cmdio"
	"github.com/spf13/cobra"
)

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
		case authsActionList:
			return invokeAuthsList(io, args.projFile)
		case authsActionShow:
			return invokeAuthsShow(io, args.projFile, args.auth, args.unmask)
		case authsActionDelete:
			return invokeAuthsDelete(io, args.projFile, args.auth, args.force)
		case authsActionNew:
			return invokeAuthsNew(io, args.projFile, args.auth, args.sets)
		case authsActionEdit:
			return invokeAuthsEdit(io, args.projFile, args.auth, args.sets, args.unmask)
		case authsActionGet:
			return invokeAuthsGet(io, args.projFile, args.auth, args.getItem, args.unmask)
		case authsActionClear:
			return invokeAuthsClear(io, args.projFile, args.auth)
		default:
			panic(fmt.Sprintf("unhandled auths action %q", args.action))
		}
	},
}

func init() {
	authsCmd.PersistentFlags().StringVarP(&flags.ProjectFile, "project-file", "F", morc.DefaultProjectPath, "Use `FILE` for project data instead of "+morc.DefaultProjectPath+".")
	authsCmd.PersistentFlags().StringVarP(&flags.New, "new", "N", "", "Create a new auth method named `AUTH`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Delete, "delete", "D", "", "Delete the auth method named `AUTH`.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BForce, "force", "f", false, "Force deletion of an auth method even if it is used in a request.")
	authsCmd.PersistentFlags().StringVarP(&flags.Get, "get", "G", "", "Get the value of the given attribute `ATTR` from the auth method. ATTR must be one of: "+strings.Join(authAttrKeyNames(), ", "))
	authsCmd.PersistentFlags().StringVarP(&flags.Clear, "clear", "C", "", "Clear any currently saved auth proof. Only applicable to auth types that use dynamically-retrieved proofs, such a cookie or a token.")
	authsCmd.PersistentFlags().StringVarP(&flags.Name, "name", "n", "", "Change the name of an auth method to `NAME`.")
	authsCmd.PersistentFlags().StringVarP(&flags.Type, "type", "T", "", "Set the type of auth method to `TYPE`. TYPE must be one of 'basic', 'session', 'jwt', or 'token'; the choice determined what other options are available.")
	authsCmd.PersistentFlags().StringVarP(&flags.Username, "username", "u", "", "Set the `USERNAME` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Password, "password", "p", "", "Set the `PASSWORD` for use with HTTP basic auth. Only valid when --type is 'basic'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Retrieval, "retrieval", "r", "", "Set the flow or request template to use to retrieve proof of authentication. This is a string of the form F:NAME for a flow or R:NAME for a request template; if no prefix is given, it is assumed to be a flow name. Only valid when --type is 'session', 'jwt', or 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Cookie, "cookie", "c", "", "Set the name of the cookie to get session information from to `NAME`. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BNoExpiration, "no-exp", "", false, "Do not detect an expiration time for the auth proof, resulting in it being used until an invalid auth is detected. Only valid when --type is 'session' or 'token'.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BYesExpiration, "exp", "", false, "Enable detection of an expiration time for a session cookie based auth proof, resulting in a new one being automatically retrieved before the authenticated request if the currently held one has expired. Only valid when --type is 'session'.")
	authsCmd.PersistentFlags().StringVarP(&flags.TokenScraper, "token-scraper", "t", "", "Set the scraper to use to extract the token from the last response of auth proof retrieval. Only valid when --type is 'jwt' or 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Dest, "dest", "d", "", "Set where the token should be used in the authenticated request. `LOCATION` must be either 'header:NAME-OF-HEADER' or 'cookie:NAME-OF-COOKIE'. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.Format, "format", "", "", "Set the format of the token proof in the authenticated to `FORMAT`. If not set, the token's exact value is used. If set to `bearer`, it's value will be preceded by the word 'Bearer'. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.ExpirationScraper, "exp-scraper", "x", "", "Set the scraper to use to extract the expiration time of the token from the last response of auth proof retrieval. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().StringVarP(&flags.ExpirationLayout, "exp-layout", "L", "RFC3339", "Set the layout of the expiration time of the token to `LAYOUT`. This can either be a custom string that is Go time layout format, or one of the following constants: 'RFC822', 'RFC822Z', 'RFC850', 'RFC1123', 'RFC1123Z', 'RFC3339', or 'RFC3339Nano'. Only valid when --type is 'token'.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BUnmask, "unmask", "", false, "Show passwords and other secrets in output. Only valid when getting or editing properties of an auth method.")
	authsCmd.PersistentFlags().BoolVarP(&flags.BQuiet, "quiet", "q", false, "Suppress all unnecessary output.")

	authsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "clear")

	// don't specify attribute args if not creating or setting.
	authsCmd.MarkFlagsMutuallyExclusive("new", "delete", "get", "clear", "name")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "type")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "username")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "password")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "retrieval")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "cookie")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "no-exp")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "exp")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "token-scraper")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "dest")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "format")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "exp-scraper")
	authsCmd.MarkFlagsMutuallyExclusive("delete", "get", "clear", "exp-layout")

	authsCmd.MarkFlagsMutuallyExclusive("new", "delete", "clear", "unmask")
	authsCmd.MarkFlagsMutuallyExclusive("new", "get", "clear", "force")
	authsCmd.MarkFlagsMutuallyExclusive("no-exp", "exp-scraper")
	authsCmd.MarkFlagsMutuallyExclusive("no-exp", "exp-layout")

	authsCmd.MarkFlagsMutuallyExclusive("no-exp", "exp")

	rootCmd.AddCommand(authsCmd)
}

func invokeAuthsList(io cmdio.IO, projFile string) error {
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	if len(p.Auths) == 0 {
		io.PrintLoudln("(none)")
	} else {
		// alphabetize the auths
		var sortedNames []string
		for name := range p.Auths {
			sortedNames = append(sortedNames, name)
		}
		sort.Strings(sortedNames)

		for _, name := range sortedNames {
			auth := p.Auths[name]

			notUsableBang := ""
			if !auth.Sendable() {
				notUsableBang = "!"
			}

			t := strings.ToUpper(string(auth.Type))
			if t == "" {
				t = "???"
			}

			io.Printf("%s:%s %s\n", auth.Name, notUsableBang, t)
		}
	}

	return nil
}

func invokeAuthsShow(io cmdio.IO, projFile, authName string, unmaskSecrets bool) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for auth template names
	authLower := strings.ToLower(authName)
	auth, ok := p.Auths[authLower]
	if !ok {
		return morc.NewReqNotFoundError(authLower)
	}

	// print out type:
	t := strings.ToUpper(string(auth.Type))
	if t == "" {
		t = "(no-type)"
	}

	io.Printf("%s\n", t)

	// layout is different based on the type of auth;
	// get values used by multiple types, then only print if relevant.

	// retrieval
	seq := auth.RetrievalSequence()
	seqLine := ""
	if seq.Name == "" && !io.Quiet {
		seqLine = "(not set)"
	} else if io.Quiet {
		seqLine = seq.String()
	} else {
		// keep flowOrReq to same number of words for each for consistency
		flowOrReq := "request"
		if seq.IsFlow {
			flowOrReq = "flow"
		}
		seqLine = fmt.Sprintf("%s %q\n", flowOrReq, seq.Name)
	}

	// currently held-value
	cached := ""
	if auth.Proof != nil {
		cached = auth.CachedValue()
		cacheEmpty := cached == ""

		if !unmaskSecrets {
			cached = strings.Repeat("*", len(cached))
		}

		// add expiration if needed
		exp := auth.CachedExpiration()
		if !cacheEmpty {
			if !exp.IsZero() {
				if io.Quiet {
					cached += fmt.Sprintf(" :%s", exp.Format(time.RFC3339))
				}
				cached += fmt.Sprintf(" :expires %s", exp.Format(time.RFC3339))
			} else {
				if io.Quiet {
					cached += " :?"
				} else {
					cached += " :expiration unknown"
				}
			}
		}
	}
	if cached == "" && !io.Quiet {
		cached = "(empty)"
	}

	// expiration detection
	expDetect := io.OnOrOff(auth.IsDetectingExpiration())

	switch auth.Type {
	case morc.AuthTypeNone:
		io.PrintLoudf("(no other attributes)\n")
	case morc.AuthTypeHTTPBasic:
		// username and password
		if auth.Proof == nil {
			io.Printf("(no credentials set)\n")
		} else {
			io.Printf("Username: ")
			user := auth.Username()
			if user == "" && !io.Quiet {
				user = "(empty)"
			} else {
				user = fmt.Sprintf("%q", user)
			}
			io.Printf("%s\n", user)

			io.Printf("Password: ")
			pass := auth.Password()
			if pass == "" && !io.Quiet {
				pass = "(empty)"
			} else {
				if !unmaskSecrets {
					pass = strings.Repeat("*", len(pass))
				}
				pass = fmt.Sprintf("%q", pass)
			}
			io.Printf("%s\n", pass)
		}
	case morc.AuthTypeSession:
		io.Printf("Retrieval: %s\n", seqLine)

		// cookie
		cookie := auth.CookieName()
		if cookie == "" && !io.Quiet {
			cookie = "(not set)"
		}
		io.Printf("Cookie: %s\n", cookie)

		io.Printf("Expiration Detection: %s\n", expDetect)
		io.Printf("Cached Proof: %s\n", cached)
	case morc.AuthTypeJWT:
		io.Printf("Retrieval: %s\n", seqLine)

		// scraper
		scraper := auth.ValueScraper()
		tokenSpec := scraper.Spec()
		if scraper.Name == "" && !io.Quiet {
			tokenSpec = "(not set)"
		}
		io.Printf("Token Scraper: %s\n", tokenSpec)

		io.Printf("Expiration Detection: %s\n", expDetect)
		io.Printf("Cached Proof: %s\n", cached)
	case morc.AuthTypeToken:
		io.Printf("Retrieval: %s\n", seqLine)

		scraper := auth.ValueScraper()
		tokenSpec := scraper.Spec()
		if scraper.Name == "" && !io.Quiet {
			tokenSpec = "(not set)"
		}
		io.Printf("Token Scraper: %s\n", tokenSpec)

		dest := auth.Destination()
		io.Printf("Token Destination: %s\n", dest.String())

		expScraper := auth.ExpirationScraper()
		expSpec := expScraper.Spec()
		if expScraper.Name == "" && !io.Quiet {
			expSpec = "(not set)"
		}
		io.Printf("Expiration Scraper: %s\n", expSpec)

		expLayout := auth.ExpirationLayout()
		if expLayout != "" {
			io.Printf("Scraped Expiration Layout: %q\n", expLayout)
		} else {
			io.Printf("Scraped Expiration Layout: (not set)\n")
		}

		io.Printf("Cached Proof: %s\n", cached)
	}

	// finally, print out if this is useable currently.
	if auth.Sendable() {
		if io.Quiet {
			io.Printf("usable\n")
		} else {
			io.PrintLoudf("Auth method is fully configured and useable.\n")
		}
	} else {
		if io.Quiet {
			io.Printf("not-usable\n")
		} else {
			io.PrintLoudf("! Auth method requires additional config before use.\n")
		}
	}

	return nil
}

func invokeAuthsClear(io cmdio.IO, projFile, authName string) error {
	// load the project file
	p, err := readProject(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for auth method names
	authLower := strings.ToLower(authName)
	auth, ok := p.Auths[authLower]
	if !ok {
		return morc.NewAuthNotFoundError(authName)
	}

	if auth.Static() {
		return fmt.Errorf("auth method %s does not cache auth proofs", auth.Name)
	}

	auth.Proof = nil

	err = writeProject(p, false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Cleared cached proof for auth method %s\n", auth.Name)

	return nil
}

func invokeAuthsGet(io cmdio.IO, projFile, authName string, item authKey, unmaskSecrets bool) error {
	// load the project file
	p, err := readProject(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for auth method names

	authLower := strings.ToLower(authName)
	auth, ok := p.Auths[authLower]
	if !ok {
		return morc.NewAuthNotFoundError(authName)
	}

	switch item {
	case authKeyName:
		io.Printf("%s\n", auth.Name)
	case authKeyType:
		if auth.Type == morc.AuthTypeNone {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", auth.Type.String())
		}
	case authKeyUsername:
		if auth.Type != morc.AuthTypeHTTPBasic {
			io.PrintLoudf("(n/a)\n")
		} else if auth.Username() == "" {
			io.PrintLoudf("(empty)\n")
		} else {
			io.Printf("%s\n", auth.Username())
		}
	case authKeyPassword:
		if auth.Type != morc.AuthTypeHTTPBasic {
			io.PrintLoudf("(n/a)\n")
		} else if auth.Password() == "" {
			io.PrintLoudf("(empty)\n")
		} else {
			pass := auth.Password()
			if !unmaskSecrets {
				pass = strings.Repeat("*", len(pass))
			}
			io.Printf("%s\n", pass)
		}
	case authKeyCookie:
		if auth.Type != morc.AuthTypeSession {
			io.PrintLoudf("(n/a)\n")
		} else if auth.CookieName() == "" {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", auth.CookieName())
		}
	case authKeyDest:
		if auth.Static() {
			io.PrintLoudf("(n/a)\n")
		} else {
			dest := auth.Destination()
			io.Printf("%s:%s\n", dest.Location.String(), dest.Key)
		}
	case authKeyExpDetection:
		if auth.Static() {
			io.PrintLoudf("(n/a)\n")
		} else {
			io.Printf("%s\n", io.OnOrOff(auth.IsDetectingExpiration()))
		}
	case authKeyExpScraper:
		if auth.Static() {
			io.PrintLoudf("(n/a)\n")
		} else if auth.ExpirationScraper().Spec() == "" {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", auth.ExpirationScraper().Spec())
		}
	case authKeyExpLayout:
		if auth.Static() {
			io.PrintLoudf("(n/a)\n")
		} else if auth.ExpirationLayout() == "" {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", auth.ExpirationLayout())
		}
	case authKeyFormat:
		if auth.Static() {
			io.PrintLoudf("(n/a)\n")
		} else if auth.Destination().Format == morc.ProofFormatTypeNone {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", auth.Destination().Format)
		}
	case authKeyRetrieval:
		if auth.Static() {
			io.PrintLoudf("(n/a)\n")
		} else if auth.RetrievalSequence().Name == "" {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", auth.RetrievalSequence().String())
		}
	case authKeyTokenScraper:
		if auth.Type != morc.AuthTypeJWT && auth.Type != morc.AuthTypeToken {
			io.PrintLoudf("(n/a)\n")
		} else if auth.ValueScraper().Spec() == "" {
			io.PrintLoudf("(none)\n")
		} else {
			io.Printf("%s\n", auth.ValueScraper().Spec())
		}
	}

	return nil
}

func invokeAuthsEdit(io cmdio.IO, projFile, authName string, attrs authAttrValues, unmaskSecrets bool) error {
	// load the project file
	p, err := readProject(projFile, false)
	if err != nil {
		return err
	}

	// case doesn't matter for auth names
	authLower := strings.ToLower(authName)
	auth, ok := p.Auths[authLower]
	if !ok {
		return morc.NewAuthNotFoundError(authName)
	}

	// check attrs for validity
	err = validateAttrCombos(attrs, &auth)
	if err != nil {
		return err
	}

	modifiedVals := map[authKey]interface{}{}
	noChangeVals := map[authKey]interface{}{}

	// if changing names, do that first
	if attrs.name.set {
		newNameLower := strings.ToLower(attrs.name.v)
		if newNameLower != authLower {
			if newNameLower == "" {
				return fmt.Errorf("new name cannot be empty")
			}
			if _, exists := p.Auths[newNameLower]; exists {
				return fmt.Errorf("auth method named %s already exists", newNameLower)
			}

			// update the name in any requests that use it
			for _, tmplName := range p.TemplatesWithAuth(authLower) {
				tmpl := p.Templates[tmplName]
				tmpl.Auth = newNameLower
				delete(p.Templates, tmplName)
				p.Templates[tmpl.Name] = tmpl
			}

			auth.Name = newNameLower
			delete(p.Auths, authLower)
			modifiedVals[authKeyName] = newNameLower
		} else {
			noChangeVals[authKeyName] = authLower
		}
	}

	// any name changes will have gone to auth.Name at this point; be shore to
	// use that for any name displaying from this point forward

	// need to do type immediately after name as we invoke SetX methods that
	// depend on the correct type being set.
	if attrs.authType.set {
		if attrs.authType.v != auth.Type {
			auth.Type = attrs.authType.v

			// this invalidates all fields besides the name
			auth.Proof = nil
			auth.Fetcher = nil // other sets will automagically refill this

			modifiedVals[authKeyType] = attrs.authType.v
		} else {
			noChangeVals[authKeyType] = auth.Type
		}
	}

	if attrs.username.set {
		if attrs.username.v != auth.Username() {
			if err := auth.SetUsername(attrs.username.v); err != nil {
				return fmt.Errorf("set username: %w", err)
			}

			modifiedVals[authKeyUsername] = attrs.username.v
		} else {
			noChangeVals[authKeyUsername] = auth.Username()
		}
	}

	if attrs.password.set {
		if attrs.password.v != auth.Password() {
			if err := auth.SetPassword(attrs.password.v); err != nil {
				return fmt.Errorf("set password: %w", err)
			}

			dispVal := attrs.password.v
			if !unmaskSecrets {
				dispVal = strings.Repeat("*", len(dispVal))
			}
			modifiedVals[authKeyPassword] = dispVal
		} else {
			dispVal := auth.Password()
			if !unmaskSecrets {
				dispVal = strings.Repeat("*", len(dispVal))
			}
			noChangeVals[authKeyPassword] = dispVal
		}
	}

	if attrs.cookie.set {
		if attrs.cookie.v != auth.CookieName() {
			if err := auth.SetCookieName(attrs.cookie.v); err != nil {
				return fmt.Errorf("set cookie name: %w", err)
			}

			modifiedVals[authKeyCookie] = attrs.cookie.v
		} else {
			noChangeVals[authKeyCookie] = auth.CookieName()
		}
	}

	if attrs.dest.set {
		// need to exclusively check the non-format part to detect for change
		compDest := attrs.dest.v
		compDest.Format = auth.Destination().Format

		if compDest != auth.Destination() {
			if err := auth.SetDestination(compDest); err != nil {
				return fmt.Errorf("set destination: %w", err)
			}

			modifiedVals[authKeyDest] = attrs.dest.v.Location.String() + ":" + attrs.dest.v.Key
		} else {
			noChangeVals[authKeyDest] = auth.Destination().Location.String() + ":" + auth.Destination().Key
		}
	}

	if attrs.format.set {
		// need to exclusively check only the format part to detect for change
		if attrs.format.v != auth.Destination().Format {
			// create the entire destination object to set it
			dest := auth.Destination()
			dest.Format = attrs.format.v

			if err := auth.SetDestination(dest); err != nil {
				return fmt.Errorf("set format: %w", err)
			}

			modifiedVals[authKeyFormat] = attrs.format.v
		} else {
			noChangeVals[authKeyFormat] = auth.Destination().Format
		}
	}

	if attrs.retrieval.set {
		if attrs.retrieval.v != auth.RetrievalSequence() {
			if err := auth.SetRetrievalSequence(attrs.retrieval.v); err != nil {
				return fmt.Errorf("set retrieval sequence: %w", err)
			}

			modifiedVals[authKeyRetrieval] = attrs.retrieval.v.String()
		} else {
			noChangeVals[authKeyRetrieval] = auth.RetrievalSequence().String()
		}
	}

	if attrs.tokenSpec.set {
		if attrs.tokenSpec.v.Spec() != auth.ValueScraper().Spec() {
			if err := auth.SetValueScraper(attrs.tokenSpec.v); err != nil {
				return fmt.Errorf("set token scraper: %w", err)
			}

			modifiedVals[authKeyTokenScraper] = attrs.tokenSpec.v.Spec()
		} else {
			noChangeVals[authKeyTokenScraper] = auth.ValueScraper().Spec()
		}
	}

	if attrs.expirationSpec.set {
		if attrs.expirationSpec.v.Spec() != auth.ExpirationScraper().Spec() {
			if err := auth.SetExpirationScraper(&attrs.expirationSpec.v); err != nil {
				return fmt.Errorf("set expiration scraper: %w", err)
			}

			modifiedVals[authKeyExpScraper] = attrs.expirationSpec.v.Spec()
		} else {
			noChangeVals[authKeyExpScraper] = auth.ExpirationScraper().Spec()
		}
	}

	if attrs.expirationLayout.set {
		if attrs.expirationLayout.v != auth.ExpirationLayout() {
			if err := auth.SetExpirationLayout(attrs.expirationLayout.v); err != nil {
				return fmt.Errorf("set expiration layout: %w", err)
			}

			modifiedVals[authKeyExpLayout] = attrs.expirationLayout.v
		} else {
			noChangeVals[authKeyExpLayout] = auth.ExpirationLayout()
		}
	}

	if attrs.expirationDetection.set {
		if attrs.expirationDetection.v != auth.IsDetectingExpiration() {
			if attrs.expirationDetection.v {
				// only auto-setting is appropriate here
				if err := auth.SetExpirationScraper(nil); err != nil {
					return fmt.Errorf("enable expiration detection: %w", err)
				}
			} else {
				// remove expiration scraper
				if err := auth.RemoveExpirationScraper(); err != nil {
					return fmt.Errorf("disable expiration detection: %w", err)
				}
			}

			modifiedVals[authKeyExpDetection] = attrs.expirationDetection.v
		} else {
			noChangeVals[authKeyExpDetection] = auth.IsDetectingExpiration()
		}
	}

	err = writeProject(p, false)
	if err != nil {
		return err
	}

	cmdio.OutputLoudEditAttrsResult(io, modifiedVals, noChangeVals, authAttrKeys)

	return nil
}

func invokeAuthsNew(io cmdio.IO, projFile, authName string, attrs authAttrValues) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// check all attrs for validity
	err = validateAttrCombos(attrs, nil)
	if err != nil {
		return err
	}

	// case doesn't matter for auth method names
	authLower := strings.ToLower(authName)
	// check if the project already has an auth method with the same name
	if _, exists := p.Auths[authLower]; exists {
		return morc.NewAuthExistsError(authLower)
	}

	if authLower == "" {
		return fmt.Errorf("name cannot be empty")
	}

	// create the new auth method. we have helper methods for each type, which
	// we will invoke now.
	auth := morc.Auth{
		Name: authName,
		Type: morc.AuthType(attrs.authType.Or(morc.AuthTypeNone)),
	}

	if auth.Type == morc.AuthTypeHTTPBasic {
		auth.Proof = morc.NewHTTPBasicCredentials(attrs.username.v, attrs.password.v)
	} else if auth.Type == morc.AuthTypeSession {
		auth.Fetcher = morc.NewSessionCookieFetcher(
			attrs.retrieval.v,
			attrs.cookie.v,
			attrs.expirationDetection.v,
		)
	} else if auth.Type == morc.AuthTypeJWT {
		auth.Fetcher = morc.NewJWTFetcher(
			attrs.retrieval.v,
			attrs.tokenSpec.v,
		)
	} else if auth.Type == morc.AuthTypeToken {
		dest := attrs.dest.v
		dest.Format = attrs.format.v

		auth.Fetcher = morc.NewTokenFetcher(
			attrs.retrieval.v,
			attrs.tokenSpec.v,
			dest,
			attrs.expirationSpec.Ptr(),
			attrs.expirationLayout.v,
		)
	}

	if p.Auths == nil {
		p.Auths = make(map[string]morc.Auth)
	}
	p.Auths[authLower] = auth

	// save the project file
	err = writeProject(p, false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Created new auth method %s\n", authLower)

	return nil
}

func invokeAuthsDelete(io cmdio.IO, projFile, authName string, force bool) error {
	// load the project file
	p, err := readProject(projFile, true)
	if err != nil {
		return err
	}

	// case doesn't matter for auth template names
	authLower := strings.ToLower(authName)
	if _, ok := p.Auths[authLower]; !ok {
		return morc.NewAuthNotFoundError(authLower)
	}

	if !force {
		// check if this method is used in any requests; cannot delete it if so
		inReqs := p.TemplatesWithAuth(authLower)

		if len(inReqs) > 0 {
			reqS := "s"
			if len(inReqs) == 1 {
				reqS = ""
			}
			return fmt.Errorf("%s is used in request template%s %s\nUse -f to force-delete", authLower, reqS, strings.Join(inReqs, ", "))
		}
	}

	// if we are forcing, there's no checks to make.

	delete(p.Auths, authLower)

	// save the project file
	err = writeProject(p, false)
	if err != nil {
		return err
	}

	io.PrintLoudf("Deleted auth method %s\n", authLower)

	return nil
}

// validateAttrs returns an error if the combination of attrs is not valid for
// either a brand new auth method or for an existing one. If existing is nil,
// the attrs will be validated as though they are creating a brand new one.
func validateAttrCombos(attrs authAttrValues, existing *morc.Auth) error {

	setType := attrs.authType.v
	if existing != nil && !attrs.authType.set {
		setType = existing.Type
	}

	var errs []error

	if attrs.username.set && setType != morc.AuthTypeHTTPBasic {
		errs = append(errs, fmt.Errorf("--username/-u is not a valid option for auth type %q", setType))
	}
	if attrs.password.set && setType != morc.AuthTypeHTTPBasic {
		errs = append(errs, fmt.Errorf("--password/-p is not a valid option for auth type %q", setType))
	}
	if attrs.cookie.set && setType != morc.AuthTypeSession {
		errs = append(errs, fmt.Errorf("--cookie/-c is not a valid option for auth type %q", setType))
	}
	if attrs.retrieval.set && setType != morc.AuthTypeSession && setType != morc.AuthTypeJWT && setType != morc.AuthTypeToken {
		errs = append(errs, fmt.Errorf("--retrieval/-r is not a valid option for auth type %q", setType))
	}
	if attrs.expirationDetection.set {
		// disabling expiration detection is valid for both session and token
		// types, but enabling is stating to use auto-detection which cannot be
		// done with token, so enabling is only valid for session.
		if attrs.expirationDetection.v && setType != morc.AuthTypeSession {
			extraTip := ""
			if setType == morc.AuthTypeToken {
				extraTip = "; use --exp-scraper to enable expiration detection"
			} else if setType == morc.AuthTypeJWT {
				extraTip = "; expiration is automatically extracted from JWT if present"
			}
			errs = append(errs, fmt.Errorf("--exp is not a valid option for auth type %q%s", setType, extraTip))
		} else if !attrs.expirationDetection.v && setType != morc.AuthTypeSession && setType != morc.AuthTypeToken {
			var extraTip string
			if setType == morc.AuthTypeJWT {
				extraTip = "; expiration is automatically extracted from JWT if present"
			}
			errs = append(errs, fmt.Errorf("--no-exp is not a valid option for auth type %q%s", setType, extraTip))
		}
	}
	if attrs.tokenSpec.set && setType != morc.AuthTypeJWT && setType != morc.AuthTypeToken {
		errs = append(errs, fmt.Errorf("--token-scraper/-t is not a valid option for auth type%q", setType))
	}
	if attrs.dest.set && setType != morc.AuthTypeToken {
		errs = append(errs, fmt.Errorf("--dest/-d is not a valid option for auth type %q", setType))
	}
	if attrs.format.set && setType != morc.AuthTypeToken {
		errs = append(errs, fmt.Errorf("--format is not a valid option for auth type %q", setType))
	}
	if attrs.expirationSpec.set && setType != morc.AuthTypeToken {
		extraTip := ""
		if setType == morc.AuthTypeSession {
			extraTip = "; use --exp to enable expiration detection"
		} else if setType == morc.AuthTypeJWT {
			extraTip = "; expiration is automatically extracted from JWT if present"
		}
		errs = append(errs, fmt.Errorf("--exp-scraper/-x is not a valid option for auth type %q%s", setType, extraTip))
	}
	if attrs.expirationLayout.set && setType != morc.AuthTypeToken {
		extraTip := ""
		if setType == morc.AuthTypeSession {
			extraTip = "; expiration format is automatically determined from cookie when present"
		} else if setType == morc.AuthTypeJWT {
			extraTip = "; expiration is automatically determined from JWT when present"
		}
		errs = append(errs, fmt.Errorf("--exp-layout/-L is not a valid option for auth type %q%s", setType, extraTip))
	}

	// join all errors together
	var sb strings.Builder
	for i, err := range errs {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(err.Error())
	}

	if sb.Len() > 0 {
		return fmt.Errorf(sb.String())
	}
	return nil
}

type authsArgs struct {
	projFile string
	action   authsAction
	getItem  authKey
	force    bool
	auth     string
	unmask   bool

	sets authAttrValues
}

type authAttrValues struct {
	name     optional[string]
	authType optional[morc.AuthType]

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

		args.unmask = flags.BUnmask
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

		args.unmask = flags.BUnmask
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
	// * --unmask with --new, --delete, and --clear
	// * --no-exp with any flag that indicates expiration detection

	// * can't really check correct type'd flags here as user might alter type
	// and it requires knowing the CURRENT type to definitively answer, so it
	// must be checked in the individual invocation functions which will have
	// read in any existing auth method.

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
		t, err := morc.ParseAuthType(flags.Type)
		if err != nil {
			return fmt.Errorf("--type/-T: invalid auth type %q; must be one of %s", flags.Type, cmdio.OxfordCommaJoin(morc.AuthTypes, "or", true))
		}
		attrs.authType = optional[morc.AuthType]{set: true, v: t}
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
			return fmt.Errorf("--token-scraper/-t: %w", err)
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
			return fmt.Errorf("--exp-scraper/-x: %w", err)
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
