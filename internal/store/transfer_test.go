package store

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransfer(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	data, err := DecodeTransfer(strings.NewReader(`{"version":1,"links":[{"alias":"docs","destination":"https://example.com","created_at":"2020-01-01T00:00:00Z"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Import(ctx, data, false, true)
	if err != nil || r.Created != 1 {
		t.Fatalf("preview: %+v %v", r, err)
	}
	links, _ := s.List(ctx, "")
	if len(links) != 0 {
		t.Fatal("preview wrote data")
	}
	r, err = s.Import(ctx, data, false, false)
	if err != nil || r.Created != 1 {
		t.Fatalf("import: %+v %v", r, err)
	}
	r, err = s.Import(ctx, data, false, false)
	if err != nil || r.Unchanged != 1 {
		t.Fatalf("repeat: %+v %v", r, err)
	}
	links, _ = s.List(ctx, "")
	original := links[0]
	data.Links[0].Destination = "https://example.org"
	r, err = s.Import(ctx, data, false, false)
	if err != nil || r.Conflicts != 1 {
		t.Fatalf("conflict: %+v %v", r, err)
	}
	r, err = s.Import(ctx, data, true, false)
	if err != nil || r.Updated != 1 {
		t.Fatalf("overwrite: %+v %v", r, err)
	}
	links, _ = s.List(ctx, "")
	if links[0].ID != original.ID || links[0].Created != original.Created || !links[0].Enabled {
		t.Fatal("local metadata changed")
	}
	var out bytes.Buffer
	if err = s.Export(ctx, &out); err != nil {
		t.Fatal(err)
	}
	exported, err := DecodeTransfer(&out)
	if err != nil || len(exported.Links) != 1 || exported.Links[0].Destination != "https://example.org" {
		t.Fatalf("round trip: %+v %v", exported, err)
	}
	data.Links = append(data.Links, TransferLink{Alias: "bad", Destination: "not a URL"})
	if _, err = s.Import(ctx, data, true, false); err == nil {
		t.Fatal("invalid document accepted")
	}
}
func TestInvalidTransfers(t *testing.T) {
	for _, input := range []string{
		`{"version":2,"links":[]}`,
		`{"version":1}`,
		`{"version":1,"links":[]} {}`,
		`{"version":1,"links":[{"alias":"x","destination":"https://example.com"},{"alias":"x","destination":"https://example.org"}]}`,
		`{"version":1,"links":[{"alias":"x","destination":"https://example.com","created_at":"yesterday"}]}`,
	} {
		if _, err := DecodeTransfer(strings.NewReader(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}
