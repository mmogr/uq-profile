// Command uq-profile opens a UQ course profile as one printable page.
//
// UQ splits a profile across tabs, so printing it gets you whichever tab
// happens to be open. This fetches the profile, unhides the rest, and opens
// it in your browser to print or save as PDF.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var version = "dev" // set by the release build

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "uq-profile:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		list    = flag.Bool("list", false, "list a course's offerings and exit")
		urlOnly = flag.Bool("url", false, "print the profile URL instead of opening it")
		out     = flag.String("o", "", "write the page here instead of a temp file")
		showVer = flag.Bool("version", false, "print the version and exit")
		campus  = flag.String("campus", "", "prefer this campus when a course runs at several")
	)
	flag.Usage = usage
	flag.Parse()

	if *showVer {
		fmt.Println("uq-profile", version)
		return nil
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return fmt.Errorf("no course code given")
	}
	code := strings.ToUpper(args[0])
	if !ValidCode(code) {
		return fmt.Errorf("%q is not a course code (expected four letters and four digits, like CSSE1001)", code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Second}

	offerings, err := Offerings(ctx, client, code)
	if err != nil {
		return err
	}

	if *list || len(args) == 1 {
		for _, o := range offerings {
			fmt.Println(o)
		}
		if !*list {
			return fmt.Errorf("pick one: uq-profile %s <year> <semester>", code)
		}
		return nil
	}

	if len(args) != 3 {
		flag.Usage()
		return fmt.Errorf("expected a year and a semester")
	}
	year, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("%q is not a year", args[1])
	}
	semester, err := strconv.Atoi(args[2])
	if err != nil || semester < 1 || semester > 2 {
		return fmt.Errorf("%q is not a semester (1 or 2)", args[2])
	}

	offering, others := Find(offerings, year, semester)
	if offering.URL == "" {
		return fmt.Errorf("no %s profile for semester %d, %d — try: uq-profile -list %s",
			code, semester, year, code)
	}
	if *campus != "" {
		for _, o := range append(others, offering) {
			if strings.EqualFold(o.Location, *campus) {
				offering = o
			}
		}
	} else if len(others) > 0 {
		fmt.Fprintf(os.Stderr, "note: also offered at %s (use -campus to choose)\n",
			strings.Join(locations(others), ", "))
	}

	if *urlOnly {
		fmt.Println(offering.URL)
		return nil
	}
	if offering.Archived() {
		fmt.Fprintln(os.Stderr, "note: this profile is on UQ's archive, which this tool can't tidy up")
		return openBrowser(offering.URL)
	}

	page, err := Fetch(ctx, client, offering)
	if err != nil {
		return err
	}

	path := *out
	if path == "" {
		path = filepath.Join(os.TempDir(),
			fmt.Sprintf("%s-%d-sem%d.html", strings.ToLower(code), year, semester))
	}
	if err := os.WriteFile(path, page, 0o644); err != nil {
		return err
	}
	if *out != "" {
		fmt.Println(path)
		return nil
	}
	return openBrowser("file://" + path)
}

func locations(offerings []Offering) []string {
	seen := map[string]bool{}
	var out []string
	for _, o := range offerings {
		if !seen[o.Location] {
			seen[o.Location] = true
			out = append(out, o.Location)
		}
	}
	return out
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		fmt.Println(target) // no browser to hand; the URL is still useful
		return nil
	}
	return cmd.Process.Release()
}

func usage() {
	fmt.Fprint(os.Stderr, `uq-profile opens a UQ course profile as one printable page.

  uq-profile CSSE1001 2025 2      open semester 2, 2025 in your browser
  uq-profile CSSE1001             list what's available
  uq-profile -url CSSE1001 2025 2 print the profile URL

`)
	flag.PrintDefaults()
}
