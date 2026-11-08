package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFilterLinksAccessAndCounts(t *testing.T) {
	yes, no := true, false
	settings := VersionedLinks{LinkSettings: LinkSettings{
		Categories: []LinkCategory{{ID: "public", Title: "Umum"}, {ID: "private", Title: "Internal"}, {ID: "disabled", Title: "Draft"}},
		Links: []ServiceLink{
			{ID: "a", CategoryID: "public", Title: "A", URL: "https://example.org/a", IsActive: true, IsPublic: &yes},
			{ID: "b", CategoryID: "private", Title: "Rahasia", Description: "metadata internal", URL: "https://example.org/private", IsActive: true, IsPublic: &no},
			{ID: "c", CategoryID: "disabled", Title: "Nonaktif", URL: "https://example.org/disabled", IsActive: false, IsPublic: &yes},
		},
	}}
	public := FilterLinks(settings, false)
	if len(public.Links) != 1 || public.Links[0].ID != "a" || len(public.Categories) != 1 {
		t.Fatalf("public result: %+v", public)
	}
	raw, _ := json.Marshal(public)
	if strings.Contains(string(raw), "Rahasia") || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "metadata internal") {
		t.Fatalf("private data leaked: %s", raw)
	}
	loggedIn := FilterLinks(settings, true)
	if len(loggedIn.Links) != 2 || len(loggedIn.Categories) != 2 {
		t.Fatalf("authenticated result: %+v", loggedIn)
	}
}

func TestPublicFlagFalseSurvivesJSON(t *testing.T) {
	var link ServiceLink
	if err := json.Unmarshal([]byte(`{"id":"private","is_public":false}`), &link); err != nil {
		t.Fatal(err)
	}
	if link.IsPublic == nil || *link.IsPublic {
		t.Fatalf("false was not retained: %+v", link)
	}
	links := []ServiceLink{link, {ID: "old"}}
	normalizeLinkVisibility(links)
	if *links[0].IsPublic || links[1].IsPublic == nil || !*links[1].IsPublic {
		t.Fatalf("visibility migration changed saved false: %+v", links)
	}
}
