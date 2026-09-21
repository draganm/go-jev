package jev

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuilder(t *testing.T) {
	req, err := NewRequest(map[string]any{"message": "Help!"}).
		Model("jev-1.13.0").
		Noul("is_urgent", "Does this convey urgency?").
		NoulWithCriteria("is_angry", "Is the customer angry?", "Hostile language", nil).
		Choice("department", "Which team?",
			Opt("technical", "Bugs"),
			Opt("billing", nil),
		).
		Choice("color", "Which color?", ChoiceOptions("red", "green")...).
		Score("frustration", "How frustrated?", "Calm", "Frustrated", map[string]any{"level": "Very angry"}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	got, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"state":{"message":"Help!"},"model":"jev-1.13.0","questions":{` +
		`"color":{"type":"choice","instructions":"Which color?","criteria":{"red":null,"green":null}},` +
		`"department":{"type":"choice","instructions":"Which team?","criteria":{"technical":"Bugs","billing":null}},` +
		`"frustration":{"type":"score","instructions":"How frustrated?","criteria":["Calm","Frustrated",{"level":"Very angry"}]},` +
		`"is_angry":{"type":"noul","instructions":"Is the customer angry?","criteria":{"true":"Hostile language"}},` +
		`"is_urgent":{"type":"noul","instructions":"Does this convey urgency?"}}}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestBuilderErrors(t *testing.T) {
	cases := map[string]struct {
		b    *RequestBuilder
		want []string
	}{
		"no questions": {
			NewRequest("s"),
			[]string{"at least one question"},
		},
		"duplicate id": {
			NewRequest("s").Noul("q", "a?").Noul("q", "b?"),
			[]string{`duplicate question id "q"`},
		},
		"empty id": {
			NewRequest("s").Noul("", "a?"),
			[]string{"id must not be empty"},
		},
		"nil question": {
			NewRequest("s").Question("q", nil),
			[]string{`question "q" is nil`},
		},
		"collects all errors": {
			NewRequest("s").Score("s", "?", "only one").Choice("c", "?"),
			[]string{`question "s": score`, `question "c": choice`},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.b.Build()
			if err == nil {
				t.Fatal("expected error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

func TestBuilderReuse(t *testing.T) {
	b := NewRequest("s")
	if _, err := b.Build(); err == nil {
		t.Fatal("expected error for empty builder")
	}
	first, err := b.Noul("a", "?").Build()
	if err != nil {
		t.Fatalf("builder stuck with earlier error: %v", err)
	}
	second, err := b.Noul("b", "?").Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Questions) != 1 || len(second.Questions) != 2 {
		t.Fatalf("first has %d questions, second %d", len(first.Questions), len(second.Questions))
	}
}
