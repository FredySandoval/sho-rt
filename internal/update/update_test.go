package update

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
		bad  bool
	}{
		{"v1.2.3", "v1.2.4", true, false}, {"v1.9.0", "v1.10.0", true, false},
		{"v2.0.0", "v1.9.9", false, false}, {"v1.0.0", "v1.0.0", false, false},
		{"dev", "v1.0.0", false, true}, {"v1.0.0", "../../bad", false, true},
	} {
		got, err := newer(tc.a, tc.b)
		if got != tc.want || (err != nil) != tc.bad {
			t.Errorf("%s -> %s: %v, %v", tc.a, tc.b, got, err)
		}
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCheck(t *testing.T) {
	oldClient, oldVersion := client, Version
	defer func() { client = oldClient; Version = oldVersion }()
	Version = "v0.1.0"
	client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v0.2.0"}`)), Header: make(http.Header)}, nil
	})}
	tag, available, err := Check(context.Background())
	if err != nil || tag != "v0.2.0" || !available {
		t.Fatalf("%s %v %v", tag, available, err)
	}
	message, err := Run(context.Background(), true)
	if err != nil || !strings.Contains(message, "Update available") {
		t.Fatalf("%s %v", message, err)
	}
}
