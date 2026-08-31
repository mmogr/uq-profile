package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestValidCode(t *testing.T) {
	for _, c := range []string{"CSSE1001", "MATH1061", "INFS1200"} {
		if !ValidCode(c) {
			t.Errorf("ValidCode(%q) = false", c)
		}
	}
	for _, c := range []string{"csse1001", "CSSE100", "CSSE10011", "CS1001", "", "CSSE-1001"} {
		if ValidCode(c) {
			t.Errorf("ValidCode(%q) = true", c)
		}
	}
}

func TestOfferings(t *testing.T) {
	page, err := os.ReadFile("testdata/course.html")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(page)
	}))
	defer srv.Close()

	got, err := offeringsFrom(t, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d offerings, want 3", len(got))
	}

	first := got[0]
	if first.Year != 2025 || first.Semester != 2 {
		t.Errorf("first offering = sem %d %d, want sem 2 2025", first.Semester, first.Year)
	}
	if first.Location != "St Lucia" || first.Mode != "In Person" {
		t.Errorf("first offering = %q/%q, want St Lucia/In Person", first.Location, first.Mode)
	}
	if !strings.HasSuffix(first.URL, "CSSE1001-60784-7560") {
		t.Errorf("first offering URL = %q", first.URL)
	}
	if got[2].Archived() != true {
		t.Error("2024 offering should be flagged as archived")
	}
}

func TestFind(t *testing.T) {
	offerings := []Offering{
		{Year: 2025, Semester: 2, Location: "St Lucia", URL: "a"},
		{Year: 2025, Semester: 2, Location: "Gatton", URL: "b"},
		{Year: 2025, Semester: 1, Location: "St Lucia", URL: "c"},
	}
	o, others := Find(offerings, 2025, 2)
	if o.URL != "a" {
		t.Errorf("Find picked %q, want a", o.URL)
	}
	if len(others) != 1 || others[0].URL != "b" {
		t.Errorf("Find returned %v as alternatives, want the Gatton one", others)
	}
	if o, _ := Find(offerings, 2027, 1); o.URL != "" {
		t.Error("Find should return nothing for a year that isn't offered")
	}
}

func TestPrintable(t *testing.T) {
	got := string(Printable([]byte(`<html><head><title>x</title></head><body>hi</body></html>`)))
	for _, want := range []string{"<base href=", ".hidden { display: block", "<title>x</title>"} {
		if !strings.Contains(got, want) {
			t.Errorf("Printable() missing %q", want)
		}
	}
	if strings.Index(got, "<base") > strings.Index(got, "<title>") {
		t.Error("base tag should come before the rest of the head")
	}

	// a page without a head still gets the styles rather than being dropped
	if !strings.Contains(string(Printable([]byte("<body>hi</body>"))), ".hidden") {
		t.Error("Printable() should cope with a page that has no head")
	}
}

// offeringsFrom points Offerings at a test server by swapping the course URL.
func offeringsFrom(t *testing.T, base string) ([]Offering, error) {
	t.Helper()
	old := courseURLFor
	courseURLFor = func(string) string { return base }
	t.Cleanup(func() { courseURLFor = old })
	return Offerings(context.Background(), http.DefaultClient, "CSSE1001")
}
