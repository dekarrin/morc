package morc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
