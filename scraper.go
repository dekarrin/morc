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
	SpecBodyJSON   SpecType = "body_json"
	SpecBodyOffset SpecType = "body_offset"
	SpecHeader     SpecType = "header"
	SpecCookie     SpecType = "cookie"
	SpecTrailer    SpecType = "trailer"
)

// TODO: make everything that takes a BodyScraper actually take a Scraper. This
// will be fairly non-trivial, make sure all types are accounted for.
type Scraper interface {
	String() string
	Spec() string
	Scrape(resp *http.Response, preReadBody []byte) (string, error)
	Type() SpecType
	EqualSpec(other Scraper) bool
	VarName() string
	Export() map[string]interface{}
}

func ImportScraper(m map[string]interface{}) (Scraper, error) {
	if rawType, ok := m["type"]; ok {
		if strType, ok := rawType.(string); ok {
			switch strings.ToLower(strType) {
			case string(SpecBodyJSON):
				return ImportBodyScraper(m)
			case string(SpecBodyOffset):
				return ImportBodyScraper(m)
			case string(SpecHeader):
				return ImportHeaderScraper(m)
			case string(SpecCookie):
				return ImportCookieScraper(m)
			default:
				return nil, fmt.Errorf("invalid type %s", strType)
			}
		} else {
			return nil, fmt.Errorf("type: must be a string, but was %T", rawType)
		}
	} else {
		// compat for existing encapsulations from v0.4.2 - all such Scrapers
		// will be of type SpecBodyOffset or SpecBodyJSON, and the actual type
		// can be determined by the encoded properties.
		return ImportBodyScraper(m)
	}
}

type BodyScraper struct {
	Name        string
	OffsetStart int
	OffsetEnd   int
	Steps       []TraversalStep // if non-nil, OffsetStart and OffsetEnd are ignored
}

func ImportBodyScraper(m map[string]interface{}) (BodyScraper, error) {
	// compatibility for existing encapsulations from v0.4.2; type-checking
	// would be at a higher layer.
	compatMap := map[string]interface{}{}
	for k, v := range m {
		if strings.EqualFold(k, "name") {
			compatMap["name"] = v
		}
		if strings.EqualFold(k, "offset_start") || strings.EqualFold(k, "OffsetStart") {
			compatMap["offset_start"] = v
		}
		if strings.EqualFold(k, "offset_end") || strings.EqualFold(k, "OffsetEnd") {
			compatMap["offset_end"] = v
		}
		if strings.EqualFold(k, "steps") {
			compatMap["steps"] = v
		}
	}
	m = compatMap

	var bs BodyScraper

	if rawName, ok := m["name"]; ok {
		if bs.Name, ok = rawName.(string); !ok {
			return bs, fmt.Errorf("name: must be a string, but was %T", rawName)
		}
	} else {
		return bs, fmt.Errorf("missing name")
	}

	if rawOffsetStart, ok := m["offset_start"]; ok {
		if offsetStart, ok := rawOffsetStart.(float64); ok {
			bs.OffsetStart = int(offsetStart)
		} else if offsetStart, ok := rawOffsetStart.(int); ok {
			bs.OffsetStart = offsetStart
		} else {
			return bs, fmt.Errorf("offset_start: must be a number, but was %T", rawOffsetStart)
		}
	} else {
		bs.OffsetStart = 0
	}

	if rawOffsetEnd, ok := m["offset_end"]; ok {
		if offsetEnd, ok := rawOffsetEnd.(float64); ok {
			bs.OffsetEnd = int(offsetEnd)
		} else if offsetEnd, ok := rawOffsetEnd.(int); ok {
			bs.OffsetEnd = offsetEnd
		} else {
			return bs, fmt.Errorf("offset_end: must be a number, but was %T", rawOffsetEnd)
		}
	} else {
		bs.OffsetEnd = 0
	}

	if stepsSlice, ok := m["steps"].([]interface{}); ok {
		// don't try to decode a nil slice
		if stepsSlice != nil {
			bs.Steps = []TraversalStep{}
			for i, rawStep := range stepsSlice {

				stepMap, ok := rawStep.(map[string]interface{})
				if !ok {
					return bs, fmt.Errorf("step[%d]: must be an object, but was %T", i, rawStep)
				}
				step, err := ImportTraversalStep(stepMap)
				if err != nil {
					return bs, fmt.Errorf("step[%d]: %w", i, err)
				}
				bs.Steps = append(bs.Steps, step)
			}
		}
	}

	return bs, nil

}

func (bs BodyScraper) Export() map[string]interface{} {
	m := map[string]interface{}{
		"name": bs.Name,
		"type": string(bs.Type()),
	}

	if bs.Type() == SpecBodyJSON {
		var stepsSlice []interface{}
		if bs.Steps != nil {
			stepsSlice = []interface{}{}
			for _, step := range bs.Steps {
				stepsSlice = append(stepsSlice, step.Export())
			}
		}
		m["steps"] = stepsSlice
	} else {
		m["offset_start"] = bs.OffsetStart
		m["offset_end"] = bs.OffsetEnd
	}

	return m
}

func (bs BodyScraper) VarName() string {
	return bs.Name
}

func (bs BodyScraper) String() string {
	s := fmt.Sprintf("%s from ", strings.ToUpper(bs.Name))
	s += bs.Spec()
	return s
}

func (bs BodyScraper) Type() SpecType {
	if len(bs.Steps) > 0 {
		return SpecBodyJSON
	}
	return SpecBodyOffset
}

func (bs BodyScraper) EqualSpec(other Scraper) bool {
	if other == nil {
		return false
	}

	var otherBodyScraper BodyScraper
	var ok bool

	if otherBodyScraper, ok = other.(BodyScraper); !ok {
		return false
	}

	if otherBodyScraper.Type() != bs.Type() {
		return false
	}

	if bs.Type() == SpecBodyJSON {
		for i := range bs.Steps {
			if bs.Steps[i] != otherBodyScraper.Steps[i] {
				return false
			}
		}
	} else if bs.Type() == SpecBodyOffset {
		if bs.OffsetStart != otherBodyScraper.OffsetStart || bs.OffsetEnd != otherBodyScraper.OffsetEnd {
			return false
		}
	}

	// not comprable
	return false
}

func (bs BodyScraper) Spec() string {
	s := ""
	if len(bs.Steps) > 0 {
		for _, step := range bs.Steps {
			s += step.String()
		}
	} else {
		if bs.OffsetStart == 0 && bs.OffsetEnd == 0 {
			s += "entire response"
		} else {
			s += fmt.Sprintf("offset %d,", bs.OffsetStart)

			if bs.OffsetEnd == 0 {
				s += "<END>"
			} else if bs.OffsetEnd < 0 {
				s += fmt.Sprintf("<END%d>", bs.OffsetEnd)
			} else {
				s += fmt.Sprintf("%d", bs.OffsetEnd)
			}
		}
	}
	return s
}

func (bs BodyScraper) Scrape(resp *http.Response, preReadBody []byte) (string, error) {
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

	if len(bs.Steps) < 1 {
		// binary offset only, just do a bounds check
		if bs.OffsetEnd > 0 && bs.OffsetEnd > len(data) {
			return "", fmt.Errorf("end offset is %d but data length is only %d", bs.OffsetEnd, len(data))
		}

		// if end is 0, return the rest of the data
		if bs.OffsetEnd == 0 {
			return string(data[bs.OffsetStart:]), nil
		} else if bs.OffsetEnd < 0 {
			// bounds check
			actualEnd := len(data) + bs.OffsetEnd
			if actualEnd < bs.OffsetStart {
				return "", fmt.Errorf("effective end offset of %d (%d) is less than start offset %d", bs.OffsetEnd, actualEnd, bs.OffsetStart)
			}
			return string(data[bs.OffsetStart:actualEnd]), nil
		} else {
			return string(data[bs.OffsetStart:bs.OffsetEnd]), nil
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
	for idx, step := range bs.Steps {
		jsonData, err = step.Traverse(jsonData)
		if err != nil {
			errSequence := ""
			for _, oldStep := range bs.Steps[:idx+1] {
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
}

// Values returned will be either the exact value or "VALUE:EXP=EXPIRATION" if
// WithExpires is true.
type CookieScraper struct {
	Name        string
	CookieName  string
	WithExpires bool
}

func (cs CookieScraper) Export() map[string]interface{} {
	return map[string]interface{}{
		"type":         string(cs.Type()),
		"name":         cs.Name,
		"cookie":       cs.CookieName,
		"with_expires": cs.WithExpires,
	}
}

func ImportCookieScraper(m map[string]interface{}) (CookieScraper, error) {
	// compatibility for existing encapsulations from v0.4.2 not needed; this
	// didn't exist then.

	var cs CookieScraper

	if rawName, ok := m["name"]; ok {
		if cs.Name, ok = rawName.(string); !ok {
			return cs, fmt.Errorf("name: must be a string, but was %T", rawName)
		}
	} else {
		return cs, fmt.Errorf("missing name")
	}

	if rawCookie, ok := m["cookie"]; ok {
		if cs.CookieName, ok = rawCookie.(string); !ok {
			return cs, fmt.Errorf("cookie: must be a string, but was %T", rawCookie)
		}
	} else {
		return cs, fmt.Errorf("missing cookie")
	}

	if rawWithExpires, ok := m["with_expires"]; ok {
		if cs.WithExpires, ok = rawWithExpires.(bool); !ok {
			return cs, fmt.Errorf("with_expires: must be a boolean, but was %T", rawWithExpires)
		}
	} else {
		return cs, fmt.Errorf("missing with_expires")
	}

	return cs, nil
}

func (cs CookieScraper) EqualSpec(other Scraper) bool {
	if other == nil {
		return false
	}

	if other.Type() != cs.Type() {
		return false
	}

	ocs, ok := other.(CookieScraper)
	if !ok {
		return false
	}

	return cs.CookieName == ocs.CookieName && cs.WithExpires == ocs.WithExpires
}

func (cs CookieScraper) Type() SpecType {
	return SpecCookie
}

func (cs CookieScraper) VarName() string {
	return cs.Name
}

func (cs CookieScraper) Spec() string {
	withExpStr := ""
	if cs.WithExpires {
		withExpStr = " (with expiration)"
	}

	return fmt.Sprintf("cookie %s%s", cs.CookieName, withExpStr)
}

func (cs CookieScraper) String() string {
	s := fmt.Sprintf("%s from ", strings.ToUpper(cs.Name))
	s += cs.Spec()
	return s
}

func (cs CookieScraper) Scrape(resp *http.Response, preReadBody []byte) (string, error) {
	var selectedCookie *http.Cookie

	cookies := resp.Cookies()
	for _, c := range cookies {
		if strings.EqualFold(c.Name, cs.CookieName) {
			selectedCookie = c
			break
		}
	}

	if selectedCookie == nil {
		return "", fmt.Errorf("cookie %s is not present in response", cs.CookieName)
	}

	val := selectedCookie.Value

	if cs.WithExpires {
		val += fmt.Sprintf(CookieScraperExpiresDelimiter+"%s", selectedCookie.Expires.Format(time.RFC3339))
	}

	return val, nil
}

// HeaderScraper supports scraping from headers and trailers.
type HeaderScraper struct {
	Name    string
	Key     string
	Index   int
	Trailer bool
}

func (hs HeaderScraper) Export() map[string]interface{} {
	return map[string]interface{}{
		"type":  string(hs.Type()),
		"name":  hs.Name,
		"key":   hs.Key,
		"index": hs.Index,
	}
}

func ImportHeaderScraper(m map[string]interface{}) (HeaderScraper, error) {
	// compatibility for existing encapsulations from v0.4.2 not needed; this
	// didn't exist then.

	var hs HeaderScraper

	if rawName, ok := m["name"]; ok {
		if hs.Name, ok = rawName.(string); !ok {
			return hs, fmt.Errorf("name: must be a string, but was %T", rawName)
		}
	} else {
		return hs, fmt.Errorf("missing name")
	}

	if rawKey, ok := m["key"]; ok {
		if hs.Key, ok = rawKey.(string); !ok {
			return hs, fmt.Errorf("key: must be a string, but was %T", rawKey)
		}
	} else {
		return hs, fmt.Errorf("missing key")
	}

	if rawIndex, ok := m["index"]; ok {
		if index, ok := rawIndex.(float64); ok {
			hs.Index = int(index)
		} else if index, ok := rawIndex.(int); ok {
			hs.Index = index
		} else {
			return hs, fmt.Errorf("index: must be a number, but was %T", rawIndex)
		}
	} else {
		return hs, fmt.Errorf("missing index")
	}

	if rawType, ok := m["type"]; ok {
		if strType, ok := rawType.(string); ok {
			if strings.EqualFold(strType, string(SpecHeader)) {
				hs.Trailer = false
			} else if strings.EqualFold(strType, string(SpecTrailer)) {
				hs.Trailer = true
			} else {
				return hs, fmt.Errorf("type: must be 'header' or 'trailer', but was %s", strType)
			}
		} else {
			return hs, fmt.Errorf("type: must be a string, but was %T", rawType)
		}
	} else {
		return hs, fmt.Errorf("missing type")
	}

	return hs, nil
}

func (hs HeaderScraper) VarName() string {
	return hs.Name
}

func (hs HeaderScraper) String() string {
	s := fmt.Sprintf("%s from ", strings.ToUpper(hs.Name))
	s += hs.Spec()
	return s
}

func (hs HeaderScraper) Spec() string {
	source := "header"
	if hs.Trailer {
		source = "trailer"
	}
	return fmt.Sprintf("%s %s[%d]", source, hs.Key, hs.Index)
}

func (hs HeaderScraper) Type() SpecType {
	if hs.Trailer {
		return SpecTrailer
	}

	return SpecHeader
}

func (hs HeaderScraper) Scrape(resp *http.Response, preReadBody []byte) (string, error) {
	var vals []string
	var source string

	if hs.Trailer {
		vals = resp.Trailer.Values(hs.Key)
		source = "trailer"
	} else {
		vals = resp.Header.Values(hs.Key)
		source = "header"
	}

	if len(vals) < 1 {
		return "", fmt.Errorf("%s %s is not present in response", source, hs.Key)
	}

	// neg header indexes specify from the end of the list
	actualIndex := hs.Index
	if actualIndex < 0 {
		actualIndex = len(vals) + actualIndex
	}

	if len(vals) <= actualIndex {
		return "", fmt.Errorf("%s %s does not have a %d'th value; only %d values are present", source, hs.Key, hs.Index, len(vals))
	}

	return vals[actualIndex], nil
}

func (hs HeaderScraper) EqualSpec(other Scraper) bool {
	if other == nil {
		return false
	}

	if other.Type() != hs.Type() {
		return false
	}

	ohs, ok := other.(HeaderScraper)
	if !ok {
		return false
	}

	return hs.Key == ohs.Key && hs.Index == ohs.Index
}
