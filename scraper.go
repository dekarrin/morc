package morc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	CookieScraperExpiresDelimiter = ":EXP="
)

type SpecType string

const (
	SpecNone       SpecType = ""
	SpecBodyJSON   SpecType = "body_json"
	SpecBodyOffset SpecType = "body_offset"
	SpecHeader     SpecType = "header"
	SpecCookie     SpecType = "cookie"
	SpecTrailer    SpecType = "trailer"
)

type Scraper struct {
	Type SpecType
	Name string

	// only used when type is SpecBodyOffset
	OffsetStart int
	OffsetEnd   int

	// only used when type is SpecBodyJSON
	Steps []TraversalStep

	// only used when type is SpecCookie
	CookieName       string
	CookieExpiration bool

	// only used when type is SpecHeader or SpecTrailer
	Key   string
	Index int
}

// IsUsable whether the Scraper's properties are set such that it could
// successfully be used to scrape a response.
func (sc Scraper) IsUsable() bool {
	switch sc.Type {
	case SpecBodyJSON:
		return len(sc.Steps) > 0
	case SpecBodyOffset:
		return true
	case SpecCookie:
		return sc.CookieName != ""
	case SpecHeader, SpecTrailer:
		return sc.Key != ""
	}

	return false
}

func (sc Scraper) String() string {
	s := fmt.Sprintf("%s from ", strings.ToUpper(sc.Name))
	s += sc.Spec()
	return s
}

func (sc Scraper) EqualSpec(other Scraper) bool {
	if other.Type != sc.Type {
		return false
	}

	if sc.Type == SpecBodyJSON {
		for i := range sc.Steps {
			if sc.Steps[i] != other.Steps[i] {
				return false
			}
		}
	} else if sc.Type == SpecBodyOffset {
		if sc.OffsetStart != other.OffsetStart || sc.OffsetEnd != other.OffsetEnd {
			return false
		}
	} else if sc.Type == SpecCookie {
		if sc.CookieName != other.CookieName || sc.CookieExpiration != other.CookieExpiration {
			return false
		}
	} else if sc.Type == SpecHeader || sc.Type == SpecTrailer {
		if sc.Key != other.Key || sc.Index != other.Index {
			return false
		}
	} else {
		// not comprable
		return false
	}

	return true
}

func (sc Scraper) Spec() string {
	s := ""

	if sc.Type == SpecNone {
		return ""
	} else if sc.Type == SpecBodyJSON {
		for _, step := range sc.Steps {
			s += step.String()
		}
	} else if sc.Type == SpecBodyOffset {
		if sc.OffsetStart == 0 && sc.OffsetEnd == 0 {
			s += "entire response"
		} else {
			s += fmt.Sprintf("offset %d,", sc.OffsetStart)

			if sc.OffsetEnd == 0 {
				s += "<END>"
			} else if sc.OffsetEnd < 0 {
				s += fmt.Sprintf("<END%d>", sc.OffsetEnd)
			} else {
				s += fmt.Sprintf("%d", sc.OffsetEnd)
			}
		}
	} else if sc.Type == SpecCookie {
		withExpStr := ""
		if sc.CookieExpiration {
			withExpStr = " (with expiration)"
		}

		return fmt.Sprintf("cookie %s%s", sc.CookieName, withExpStr)
	} else if sc.Type == SpecHeader {
		return fmt.Sprintf("header %s[%d]", sc.Key, sc.Index)
	} else if sc.Type == SpecTrailer {
		return fmt.Sprintf("trailer %s[%d]", sc.Key, sc.Index)
	} else {
		return "unknown"
	}
	return s
}

func (sc Scraper) Scrape(resp *http.Response, preReadBody []byte) (string, error) {
	if sc.Type == SpecBodyJSON || sc.Type == SpecBodyOffset {
		var data []byte

		if preReadBody == nil {
			var err error
			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return "", fmt.Errorf("read response body: %w", err)
			}
			resp.Body.Close()

			bufData := make([]byte, len(data))
			copy(bufData, data)
			resp.Body = io.NopCloser(bytes.NewBuffer(bufData))
		} else {
			data = preReadBody
		}

		if sc.Type == SpecBodyOffset {
			// binary offset only, just do a bounds check
			if sc.OffsetEnd > 0 && sc.OffsetEnd > len(data) {
				return "", fmt.Errorf("end offset is %d but data length is only %d", sc.OffsetEnd, len(data))
			}

			// if end is 0, return the rest of the data
			if sc.OffsetEnd == 0 {
				return string(data[sc.OffsetStart:]), nil
			} else if sc.OffsetEnd < 0 {
				// bounds check
				actualEnd := len(data) + sc.OffsetEnd
				if actualEnd < sc.OffsetStart {
					return "", fmt.Errorf("effective end offset of %d (%d) is less than start offset %d", sc.OffsetEnd, actualEnd, sc.OffsetStart)
				}
				return string(data[sc.OffsetStart:actualEnd]), nil
			} else {
				return string(data[sc.OffsetStart:sc.OffsetEnd]), nil
			}
		}

		// otherwise, perform the traversal. hopefully we got either a JSON map or a
		// JSON list or this is going to fail
		var jsonData interface{}

		// ...just look ahead and check if the first non-whitespace char is a '{' or '['
		var firstChar rune
		for _, b := range data {
			if unicode.IsSpace(rune(b)) {
				continue
			}
			firstChar = rune(b)
			break
		}

		// if first char is a '{', assume it's a map
		if firstChar == '{' {
			var jsonMap map[string]interface{}
			err := json.Unmarshal(data, &jsonMap)
			if err != nil {
				return "", fmt.Errorf("unmarshal JSON map: %w", err)
			}
			jsonData = jsonMap
		} else if firstChar == '[' {
			var jsonList []interface{}
			err := json.Unmarshal(data, &jsonList)
			if err != nil {
				return "", fmt.Errorf("unmarshal JSON list: %w", err)
			}
			jsonData = jsonList
		} else {
			return "", fmt.Errorf("data does not appear to be a JSON array or object")
		}

		// now that we have the parsed data, apply traversal steps
		var err error
		for idx, step := range sc.Steps {
			jsonData, err = step.Traverse(jsonData)
			if err != nil {
				errSequence := ""
				for _, oldStep := range sc.Steps[:idx+1] {
					errSequence += oldStep.String()
				}
				return "", fmt.Errorf("traversal error at %s: %w", errSequence, err)
			}
		}

		// assuming successful traversal, jsonData should be the value we want.
		switch typedData := jsonData.(type) {
		case string:
			return typedData, nil
		default:
			return fmt.Sprintf("%v", jsonData), nil
		}
	} else if sc.Type == SpecCookie {
		var selectedCookie *http.Cookie

		cookies := resp.Cookies()
		for _, c := range cookies {
			if strings.EqualFold(c.Name, sc.CookieName) {
				selectedCookie = c
				break
			}
		}

		if selectedCookie == nil {
			return "", fmt.Errorf("cookie %s is not present in response", sc.CookieName)
		}

		val := selectedCookie.Value

		if sc.CookieExpiration {
			val += fmt.Sprintf(CookieScraperExpiresDelimiter+"%s", selectedCookie.Expires.Format(time.RFC3339))
		}

		return val, nil
	} else if sc.Type == SpecHeader || sc.Type == SpecTrailer {
		var vals []string
		var source string

		if sc.Type == SpecTrailer {
			vals = resp.Trailer.Values(sc.Key)
			source = "trailer"
		} else {
			vals = resp.Header.Values(sc.Key)
			source = "header"
		}

		if len(vals) < 1 {
			return "", fmt.Errorf("%s %s is not present in response", source, sc.Key)
		}

		// neg header indexes specify from the end of the list
		actualIndex := sc.Index
		if actualIndex < 0 {
			actualIndex = len(vals) + actualIndex
		}

		if len(vals) <= actualIndex {
			return "", fmt.Errorf("%s %s does not have a %d'th value; only %d values are present", source, sc.Key, sc.Index, len(vals))
		}

		return vals[actualIndex], nil
	} else {
		return "", fmt.Errorf("unsupported scraper type %s", sc.Type)
	}
}

// Spec format:
// [TYPE:]ARGS
//
// If TYPE: is omitted, and ARGS is not a shorthand name, the spec is assumed to
// be a JSON traversal. Disambiguate from a shorthand name by prefixing with a
// period character.
//
// JSON:.key1.key2[index]...
// BYTES:START,END
// HEADER:KEY
// HEADER:KEY[INDEX]
// TRAILER:KEY
// TRAILER:KEY[INDEX]
// COOKIE:NAME
// COOKIE:NAME,NO-EXP
// :RAW
func ParseVarScraperSpec(name, s string) (Scraper, error) {
	specType, rest := stripSpecType(strings.TrimSpace(s))

	rest = strings.TrimSpace(rest)

	// if we don't have a type, might still be okay, just check for shorthand
	if specType == SpecNone && strings.HasPrefix(s, ":") {
		return specFromShorthand(name, rest)
	}

	var vs, err = Scraper{}, error(nil)
	switch specType {
	case SpecBodyJSON:
		vs, err = parseJSONScraperSpec(name, rest)
	case SpecBodyOffset:
		vs, err = parseOffsetScraperSpec(name, rest)
	case SpecHeader:
		vs, err = parseMarginalsScraperSpec(name, rest, true)
	case SpecTrailer:
		vs, err = parseMarginalsScraperSpec(name, rest, false)
	case SpecCookie:
		vs, err = parseCookieScraperSpec(name, rest)
	case SpecNone:
		err = fmt.Errorf("spec type not specified")
	default:
		err = fmt.Errorf("unknown spec type %q", specType)
	}

	if err != nil {
		return Scraper{}, fmt.Errorf("%q: %w", s, err)
	}
	return vs, nil
}

func ParseVarName(name string) (string, error) {
	// validate that name does not contain any invalid characters; it must be
	// alphanumeric or underscore
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("name is empty")
	}
	if !regexp.MustCompile(`^` + varNamePattern + `$`).MatchString(name) {
		return "", fmt.Errorf("name %q contains invalid characters", name)
	}

	return name, nil
}

func ParseVarScraper(s string) (Scraper, error) {
	// Parse var scraper specification strings of the form "NAME::START,END" for
	// byte offsets and "NAME:key1.key2[index1]...keyN" for JSON traversal with array
	// indexes and object keys in a syntax similar to jq.

	// first, split name from spec:
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return Scraper{}, fmt.Errorf("not in NAME:SPEC format")
	}

	name, err := ParseVarName(parts[0])
	if err != nil {
		return Scraper{}, err
	}
	spec := parts[1]

	return ParseVarScraperSpec(name, spec)
}

func parseMarginalsScraperSpec(name, spec string, header bool) (Scraper, error) {
	// it is a header/trailer key, optionally with an index
	// KEY[INDEX]
	key := spec
	var index int

	marginal := SpecHeader
	if !header {
		marginal = SpecTrailer
	}

	// split on any [
	parts := strings.SplitN(spec, "[", 2)
	if len(parts) == 2 {
		key = strings.TrimSpace(parts[0])

		unparsedIndex := strings.TrimSpace(parts[1])
		if !strings.HasSuffix(unparsedIndex, "]") {
			return Scraper{}, fmt.Errorf("missing closing ] for index")
		}
		unparsedIndex = unparsedIndex[:len(unparsedIndex)-1]
		if unparsedIndex == "" {
			return Scraper{}, fmt.Errorf("missing index")
		}
		var err error
		index, err = strconv.Atoi(unparsedIndex)
		if err != nil {
			return Scraper{}, fmt.Errorf("invalid index: %w", err)
		}
	}

	if key == "" {
		return Scraper{}, fmt.Errorf("%v key is empty", marginal)
	}

	return Scraper{
		Name:  name,
		Type:  marginal,
		Key:   key,
		Index: index,
	}, nil
}

func parseCookieScraperSpec(name, spec string) (Scraper, error) {
	// it is a cookie name, optionally with :NO-EXP to indicate that expiration
	// should not be included
	parts := strings.SplitN(spec, ",", 2)
	cookieName := strings.TrimSpace(parts[0])
	if cookieName == "" {
		return Scraper{}, fmt.Errorf("cookie name is empty")
	}
	cookieExpiration := true
	if len(parts) == 2 {
		if strings.EqualFold(strings.TrimSpace(parts[1]), "NO-EXP") {
			cookieExpiration = false
		} else {
			return Scraper{}, fmt.Errorf("invalid cookie expiration spec %q", parts[1])
		}
	}
	return Scraper{
		Name:             name,
		Type:             SpecCookie,
		CookieName:       cookieName,
		CookieExpiration: cookieExpiration,
	}, nil
}

func parseOffsetScraperSpec(name, spec string) (Scraper, error) {
	// it is a byte offset of the form "START,END"
	offsets := strings.SplitN(spec, ",", 2)
	if len(offsets) != 2 {
		return Scraper{}, fmt.Errorf("not in START,END format")
	}

	var start, end int
	var err error

	if len(offsets[0]) > 0 {
		start, err = strconv.Atoi(offsets[0])
		if err != nil {
			return Scraper{}, fmt.Errorf("start offset: %w", err)
		}

		if start < 0 {
			return Scraper{}, fmt.Errorf("start offset cannot be negative")
		}
	}

	if len(offsets[1]) > 0 {
		end, err = strconv.Atoi(offsets[1])
		if err != nil {
			return Scraper{}, fmt.Errorf("end offset: %w", err)
		}
	}

	// only matters if end is greater than 0; 0 means "to the end", -1 means 1 from the end, etc.
	if end <= start && end > 0 {
		return Scraper{}, fmt.Errorf("end offset %d is less than or equal to start offset %d", end, start)
	}

	return Scraper{
		Name:        name,
		Type:        SpecBodyOffset,
		OffsetStart: start,
		OffsetEnd:   end,
	}, nil
}

func parseJSONScraperSpec(name, spec string) (Scraper, error) {
	steps := []TraversalStep{}
	var currentStep TraversalStep

	type mode int64

	const (
		none mode = iota
		inKey
		inQuotedKey
		inIndex
	)

	var curMode mode

	var curSymbol strings.Builder

	specR := []rune(spec)
	for i := 0; i < len(specR); i++ {
		ch := specR[i]

		switch curMode {
		case none:
			if ch == '.' {
				// lookahead to see if in quote
				if i+1 < len(specR) && specR[i+1] == '"' {
					curMode = inQuotedKey
					i++
				} else {
					curMode = inKey
				}
			} else if ch == '[' {
				curMode = inIndex
			} else {
				return Scraper{}, fmt.Errorf("invalid character %q at position %d; should be either '.' to specify a key or '[' to specify an index", ch, i)
			}
		case inKey:
			if ch == '.' || ch == '[' {
				// at end of the key, add it to the steps, reset mode, and continue
				// parsing at this index
				symStr := curSymbol.String()
				if symStr == "" {
					return Scraper{}, fmt.Errorf("missing key at position %d", i)
				}
				currentStep.Key = symStr
				steps = append(steps, currentStep)
				currentStep = TraversalStep{}
				curSymbol.Reset()
				curMode = none
				i--
			} else if ch == '\\' {
				// escape character; consume next character
				i++
				if i >= len(specR) {
					return Scraper{}, fmt.Errorf("escape character at end of string")
				}
				curSymbol.WriteRune(specR[i])
			} else if unicode.IsSpace(ch) {
				return Scraper{}, fmt.Errorf("unescaped whitespace character in key at position %d; quote key name or escape whitespace with '\\'", i)
			} else {
				curSymbol.WriteRune(ch)
			}
		case inQuotedKey:
			if ch == '"' {
				// end of quoted key
				symStr := curSymbol.String()
				if symStr == "" {
					return Scraper{}, fmt.Errorf("missing key at position %d", i)
				}
				currentStep.Key = symStr
				steps = append(steps, currentStep)
				currentStep = TraversalStep{}
				curSymbol.Reset()
				curMode = none
			} else if ch == '\\' {
				// escape character; consume next character
				i++
				if i >= len(specR) {
					return Scraper{}, fmt.Errorf("escape character at end of string")
				}
				curSymbol.WriteRune(specR[i])
			} else {
				curSymbol.WriteRune(ch)
			}
		case inIndex:
			if ch == ']' {
				// end of index
				symStr := curSymbol.String()
				if symStr == "" {
					return Scraper{}, fmt.Errorf("missing index at position %d", i)
				}
				index, err := strconv.Atoi(symStr)
				if err != nil {
					return Scraper{}, fmt.Errorf("invalid index %q: %w", symStr, err)
				}
				currentStep.Index = index
				steps = append(steps, currentStep)
				currentStep = TraversalStep{}
				curSymbol.Reset()
				curMode = none
			} else {
				curSymbol.WriteRune(ch)
			}
		default:
			// should never happen
			return Scraper{}, fmt.Errorf("invalid mode %d", curMode)
		}
	}

	// we should be in mode none at the end, but it is valid to be in mode inKey
	// as well
	if curMode == inKey {
		symStr := curSymbol.String()
		if symStr == "" {
			return Scraper{}, fmt.Errorf("missing key at end of string")
		}
		currentStep.Key = symStr
		steps = append(steps, currentStep)
	} else if curMode == inQuotedKey {
		return Scraper{}, fmt.Errorf("unterminated quoted key at end of string")
	} else if curMode == inIndex {
		return Scraper{}, fmt.Errorf("unterminated index at end of string")
	}

	return Scraper{
		Type:  SpecBodyJSON,
		Name:  name,
		Steps: steps,
	}, nil
}

func specFromShorthand(name, s string) (Scraper, error) {
	switch strings.ToLower(s) {
	case "raw":
		return Scraper{
			Type: SpecBodyOffset,
			Name: name,
		}, nil
	default:
		return Scraper{}, fmt.Errorf("invalid var scraper spec %q", s)
	}
}

func stripSpecType(s string) (st SpecType, rest string) {
	sUpper := strings.ToUpper(s)
	if sUpper == "" {
		return SpecNone, ""
	} else if strings.HasPrefix(sUpper, ":") {
		return SpecNone, s[len(":"):]
	} else if strings.HasPrefix(sUpper, "JSON:") {
		return SpecBodyJSON, s[len("JSON:"):]
	} else if strings.HasPrefix(sUpper, "BYTES:") {
		return SpecBodyOffset, s[len("BYTES:"):]
	} else if strings.HasPrefix(sUpper, "HEADER:") {
		return SpecHeader, s[len("HEADER:"):]
	} else if strings.HasPrefix(sUpper, "TRAILER:") {
		return SpecTrailer, s[len("TRAILER:"):]
	} else if strings.HasPrefix(sUpper, "COOKIE:") {
		return SpecCookie, s[len("COOKIE:"):]
	} else {
		return SpecBodyJSON, s
	}
}
