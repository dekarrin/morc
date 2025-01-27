package morc

import (
	"fmt"
	"net/http"
	"strings"
)

type SpecType int

const (
	SpecBodyJSON SpecType = iota
	SpecBodyOffset
	SpecHeader
	SpecCookie
	SpecTrailer
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
}

// HeaderScraper supports scraping from headers and trailers.
type HeaderScraper struct {
	Name    string
	Key     string
	Index   int
	Trailer bool
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
