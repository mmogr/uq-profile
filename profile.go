package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const (
	courseURL  = "https://programs-courses.uq.edu.au/course.html?course_code=%s"
	profileURL = "https://course-profiles.uq.edu.au/"
	userAgent  = "uq-profile (https://github.com/mmogr/uq-profile)"
)

// Offering is one delivery of a course, and the profile UQ published for it.
type Offering struct {
	Period   string // as printed, e.g. "Semester 2, 2025"
	Year     int
	Semester int // 1, 2, or 0 for summer and anything else
	Location string
	Mode     string
	URL      string
}

func (o Offering) String() string {
	return fmt.Sprintf("%-22s %-12s %-12s %s", o.Period, o.Location, o.Mode, o.URL)
}

// Archived reports whether UQ moved this profile to the old system, which
// serves a different page structure that we can't make printable.
func (o Offering) Archived() bool {
	return strings.Contains(o.URL, "archive.course-profiles")
}

var (
	codePattern   = regexp.MustCompile(`^[A-Z]{4}[0-9]{4}$`)
	periodPattern = regexp.MustCompile(`Semester (\d), (\d{4})`)
	yearPattern   = regexp.MustCompile(`(\d{4})`)
)

// ValidCode reports whether s looks like a UQ course code (CSSE1001).
func ValidCode(s string) bool { return codePattern.MatchString(s) }

// swapped out in tests
var courseURLFor = func(code string) string { return fmt.Sprintf(courseURL, code) }

// Offerings lists every offering UQ shows for a course, newest first as
// published. The course page is the only place the profile URLs exist —
// they carry per-offering ids that can't be derived from the code alone.
func Offerings(ctx context.Context, c *http.Client, code string) ([]Offering, error) {
	doc, err := fetchHTML(ctx, c, courseURLFor(code))
	if err != nil {
		return nil, err
	}

	var out []Offering
	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "tr" {
			return
		}
		link := find(n, func(c *html.Node) bool {
			return c.Type == html.ElementNode && c.Data == "a" &&
				strings.Contains(attr(c, "href"), "course-profiles")
		})
		if link == nil {
			return
		}
		var cells []string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
				cells = append(cells, strings.TrimSpace(text(c)))
			}
		}
		if len(cells) < 3 {
			return
		}
		o := Offering{
			Period:   strings.TrimSpace(strings.Split(cells[0], "(")[0]),
			Location: cells[1],
			Mode:     cells[2],
			URL:      abs(attr(link, "href")),
		}
		if m := periodPattern.FindStringSubmatch(o.Period); m != nil {
			o.Semester, _ = strconv.Atoi(m[1])
			o.Year, _ = strconv.Atoi(m[2])
		} else if m := yearPattern.FindStringSubmatch(o.Period); m != nil {
			o.Year, _ = strconv.Atoi(m[1])
		}
		out = append(out, o)
	})

	if len(out) == 0 {
		return nil, fmt.Errorf("no offerings found for %s (is the code right?)", code)
	}
	return out, nil
}

// Find returns the offering for a year and semester. Courses taught at more
// than one campus have several, so the first match wins and the rest come
// back for the caller to mention.
func Find(offerings []Offering, year, semester int) (Offering, []Offering) {
	var matches []Offering
	for _, o := range offerings {
		if o.Year == year && o.Semester == semester {
			matches = append(matches, o)
		}
	}
	if len(matches) == 0 {
		return Offering{}, nil
	}
	return matches[0], matches[1:]
}

// Fetch downloads a profile page and makes it printable in one piece.
func Fetch(ctx context.Context, c *http.Client, o Offering) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profile page returned %s", resp.Status)
	}
	page, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return Printable(page), nil
}

// injected unhides the sections and drops the site chrome. UQ tabs the
// profile by giving the sections you aren't viewing class="hidden", which
// is the whole reason printing a profile normally gets you one section.
const injected = `<base href="` + profileURL + `">
<style>
  .hidden { display: block !important; }
  header, footer, nav, .uq-tabs__list, .uq-breadcrumb, .skip-link,
  #block-mainnavigation, #onetrust-consent-sdk, .back-to-top { display: none !important; }
  body { background: #fff !important; }
  main, #main-content { max-width: none !important; margin: 0 !important; }
  h2 { margin-top: 1.4em; padding-top: .6em; border-top: 1px solid #ddd; }
  table { width: 100% !important; font-size: 9pt; }
  @media print { a { color: #000; text-decoration: none; } @page { margin: 1.2cm; } }
</style>`

// Printable rewrites a profile page so every section is visible.
func Printable(page []byte) []byte {
	s := string(page)
	if i := strings.Index(s, "<head>"); i >= 0 {
		return []byte(s[:i+len("<head>")] + injected + s[i+len("<head>"):])
	}
	return append([]byte(injected), page...)
}

func fetchHTML(ctx context.Context, c *http.Client, url string) (*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("course page returned %s", resp.Status)
	}
	return html.Parse(resp.Body)
}

func abs(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return strings.TrimSuffix(profileURL, "/") + href
}

func walk(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func find(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if pred(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if got := find(c, pred); got != nil {
			return got
		}
	}
	return nil
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func text(n *html.Node) string {
	var b strings.Builder
	walk(n, func(c *html.Node) {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	})
	return strings.Join(strings.Fields(b.String()), " ")
}
