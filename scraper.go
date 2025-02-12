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

	if sc.Type == SpecBodyJSON {
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

func ParseVarScraperSpec(name, spec string) (Scraper, error) {
	// TODO: support for anything besides body-based scrapers
	// okay, are we looking at a byte offset or a JSON traversal?
	if strings.HasPrefix(spec, ":") {
		// it is a byte offset of the form ":START,END"
		offsets := strings.SplitN(spec[1:], ",", 2)
		if len(offsets) != 2 {
			return Scraper{}, fmt.Errorf("%q is not in :START,END format", spec)
		}

		var start, end int
		var err error

		if len(offsets[0]) > 0 {
			start, err = strconv.Atoi(offsets[0])
			if err != nil {
				return Scraper{}, fmt.Errorf("%q: start offset: %w", spec, err)
			}

			if start < 0 {
				return Scraper{}, fmt.Errorf("%q: start offset cannot be negative", spec)
			}
		}

		if len(offsets[1]) > 0 {
			end, err = strconv.Atoi(offsets[1])
			if err != nil {
				return Scraper{}, fmt.Errorf("%q: end offset: %w", spec, err)
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

	// otherwise, . indicates a JSON traversal. Use . as the path separator, and
	// [index] to for array indexes. Space chars and dots in keys are only
	// allowed if key is quoted with double-quotes. Unquoted keys can contain
	// any other character. Indexes must be integers. Quoted keys may contain a
	// backslash to escape a quote or backslash.

	// to make it easier, ensure that spec starts with a dot.
	if strings.HasPrefix(spec, ".") {

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

	// else, check shorthand names for captures
	switch strings.ToLower(spec) {
	case "raw":
		return Scraper{
			Type: SpecBodyOffset,
			Name: name,
		}, nil
	default:
		return Scraper{}, fmt.Errorf("invalid var scraper spec %q", spec)
	}
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
