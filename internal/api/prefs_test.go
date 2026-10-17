package api

import "testing"

func TestPrefsValidate(t *testing.T) {
	ok := prefs{Home: []homeRow{
		{ID: "continue"},
		{ID: "recent:1", Hidden: true, Title: "ignored", Filter: &rowFilter{LibraryID: 1}},
		{ID: "custom:a", Title: " Short films ", Filter: &rowFilter{LibraryID: 1, MaxRuntime: 90, Sort: "shuffle"}},
	}}
	if err := ok.validate(); err != nil {
		t.Fatal(err)
	}
	if ok.Home[1].Title != "" || ok.Home[1].Filter != nil {
		t.Error("a built-in row kept a title or filter")
	}
	if ok.Home[2].Title != "Short films" {
		t.Errorf("title not trimmed: %q", ok.Home[2].Title)
	}

	for name, p := range map[string]prefs{
		"duplicate id":   {Home: []homeRow{{ID: "continue"}, {ID: "continue"}}},
		"empty id":       {Home: []homeRow{{ID: " "}}},
		"no title":       {Home: []homeRow{{ID: "custom:a", Filter: &rowFilter{LibraryID: 1}}}},
		"no library":     {Home: []homeRow{{ID: "custom:a", Title: "x", Filter: &rowFilter{}}}},
		"no filter":      {Home: []homeRow{{ID: "custom:a", Title: "x"}}},
		"bad sort":       {Home: []homeRow{{ID: "custom:a", Title: "x", Filter: &rowFilter{LibraryID: 1, Sort: "nope"}}}},
		"rating over 10": {Home: []homeRow{{ID: "custom:a", Title: "x", Filter: &rowFilter{LibraryID: 1, MinRating: 11}}}},
		"unknown theme":  {Theme: "pink"},
	} {
		if err := p.validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
