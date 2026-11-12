package gsc

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"go-server/internal/domain/capture"
)

func TestPerformanceURLDomainProperty(t *testing.T) {
	got := PerformanceURL("sc-domain:example.com", 28)

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if u.Host != "search.google.com" {
		t.Fatalf("host = %q", u.Host)
	}
	if u.Path != PerformancePath {
		t.Fatalf("path = %q, want %q", u.Path, PerformancePath)
	}
	if got := u.Query().Get("resource_id"); got != "sc-domain:example.com" {
		t.Fatalf("resource_id = %q", got)
	}
	if got := u.Query().Get("num_of_days"); got != "28" {
		t.Fatalf("num_of_days = %q", got)
	}
}

func TestPerformanceURLPrefixPropertyIsVerbatim(t *testing.T) {
	// URL-prefix properties are exact strings: scheme, www, trailing slash all
	// matter. The deep link must round-trip the value unchanged.
	const property = "https://www.example.com/"
	got := PerformanceURL(property, 7)

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if got := u.Query().Get("resource_id"); got != property {
		t.Fatalf("resource_id = %q, want %q", got, property)
	}
}

func TestPerformanceURLDefaultsDays(t *testing.T) {
	u, err := url.Parse(PerformanceURL("sc-domain:example.com", 0))
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if got := u.Query().Get("num_of_days"); got != "28" {
		t.Fatalf("num_of_days = %q, want 28", got)
	}
}

func TestRPCIDsExtractsAndDedupes(t *testing.T) {
	body := ")]}'\n\n123\n" +
		`[["wrb.fr","OLiH4d","[[...]]",null,null,null,"generic"],` +
		`["wrb.fr","nDAfwb","[]",null,null,null,"generic"],` +
		`["wrb.fr","OLiH4d","[]",null,null,null,"generic"]]`

	got := RPCIDs(body)
	want := []string{"OLiH4d", "nDAfwb"}
	if len(got) != len(want) {
		t.Fatalf("RPCIDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RPCIDs() = %v, want %v", got, want)
		}
	}
}

func TestRPCIDsEmptyForNonBatchBody(t *testing.T) {
	if got := RPCIDs(`{"rows":[]}`); len(got) != 0 {
		t.Fatalf("RPCIDs() = %v, want empty", got)
	}
}

func TestStripBatchPrefix(t *testing.T) {
	got := StripBatchPrefix(")]}'\n\n[[]]")
	if got != "[[]]" {
		t.Fatalf("StripBatchPrefix() = %q", got)
	}

	plain := `{"a":1}`
	if got := StripBatchPrefix(plain); got != plain {
		t.Fatalf("StripBatchPrefix() changed a body without the prefix: %q", got)
	}
}

type fakeSender struct {
	got  *capture.BrowserRequest
	resp *capture.BrowserResponse
	err  error
}

func (f *fakeSender) ResolveBrowserConnID(string) (string, error) { return "conn-1", nil }

func (f *fakeSender) SendMessage(_ context.Context, req *capture.BrowserRequest, _ string) (*capture.BrowserResponse, error) {
	f.got = req
	return f.resp, f.err
}

func TestRecordPerformanceUsesBrowserOnlyCaptureParams(t *testing.T) {
	fs := &fakeSender{resp: &capture.BrowserResponse{
		Success: true,
		Result: capture.PageResult{
			Items: []any{
				map[string]any{
					"url":       "https://search.google.com/search-console/_/PerformanceUi/data/batchexecute",
					"status":    float64(200),
					"body":      `)]}'` + "\n" + `[["wrb.fr","OLiH4d","[]"]]`,
					"bodyBytes": float64(40),
					"ts":        float64(1),
				},
			},
		},
	}}

	res, err := RecordPerformance(context.Background(), fs, nil, "sc-domain:example.com", Options{Days: 7})
	if err != nil {
		t.Fatalf("RecordPerformance() error = %v", err)
	}
	if len(res.Captures) != 1 {
		t.Fatalf("captures = %d, want 1", len(res.Captures))
	}
	if got := res.Captures[0].RPCIDs; len(got) != 1 || got[0] != "OLiH4d" {
		t.Fatalf("rpc ids = %v", got)
	}

	if fs.got == nil {
		t.Fatal("no browser request was sent")
	}
	if fs.got.Command != "intercept" {
		t.Fatalf("command = %q, want intercept", fs.got.Command)
	}
	if !strings.Contains(fs.got.URL, "resource_id=sc-domain%3Aexample.com") {
		t.Fatalf("page url = %q", fs.got.URL)
	}
	for _, key := range []string{"patterns", "waitForRequest", "exactHost", "scrollRounds", "includeBodies"} {
		if _, ok := fs.got.Params[key]; !ok {
			t.Fatalf("params missing %q: %+v", key, fs.got.Params)
		}
	}
	if fs.got.Params["exactHost"] != true {
		t.Fatalf("exactHost = %v, want true", fs.got.Params["exactHost"])
	}
	if fs.got.Params["scrollRounds"] != 0 {
		t.Fatalf("scrollRounds = %v, want 0", fs.got.Params["scrollRounds"])
	}
}

func TestRecordPerformanceEmptyCaptureIsExplicitError(t *testing.T) {
	fs := &fakeSender{resp: &capture.BrowserResponse{Success: true}}

	_, err := RecordPerformance(context.Background(), fs, nil, "sc-domain:example.com", Options{})
	if err == nil {
		t.Fatal("expected an error when nothing was captured")
	}
	if !strings.Contains(err.Error(), "REQUEST_NOT_SEEN") {
		t.Fatalf("error = %v, want REQUEST_NOT_SEEN", err)
	}
}

func TestRecordPerformanceRequiresProperty(t *testing.T) {
	fs := &fakeSender{}
	if _, err := RecordPerformance(context.Background(), fs, nil, "  ", Options{}); err == nil {
		t.Fatal("expected an error for empty property")
	}
	if fs.got != nil {
		t.Fatal("must not touch the browser without a property")
	}
}
