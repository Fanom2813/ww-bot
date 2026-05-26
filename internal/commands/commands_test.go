package commands

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		kind Kind
	}{
		{"/pause", Pause},
		{"/resume", Resume},
		{"/status", Status},
		{"/help", Help},
		{"/today deep work, no meetings", Today},
		{"just a normal note", Note},
		{"", Note},
		{"/frobnicate", Unknown},
	}
	for _, c := range cases {
		if got := Parse(c.in).Kind; got != c.kind {
			t.Errorf("Parse(%q).Kind = %q, want %q", c.in, got, c.kind)
		}
	}
}

func TestParseToday(t *testing.T) {
	cmd := Parse("/today  busy all afternoon")
	if cmd.Kind != Today || cmd.Text != "busy all afternoon" {
		t.Fatalf("today parse: %+v", cmd)
	}
}

func TestParseTier(t *testing.T) {
	cmd := Parse("/tier Dad auto")
	if cmd.Kind != Tier || cmd.Target != "Dad" || cmd.Tier != "auto" {
		t.Fatalf("tier parse: %+v", cmd)
	}

	// Multi-word target.
	cmd = Parse("/tier Uncle Sam draft")
	if cmd.Kind != Tier || cmd.Target != "Uncle Sam" || cmd.Tier != "draft" {
		t.Fatalf("multiword tier: %+v", cmd)
	}

	// Invalid tier -> Unknown.
	if Parse("/tier Dad sometimes").Kind != Unknown {
		t.Fatal("invalid tier should be Unknown")
	}

	// Missing args -> Unknown.
	if Parse("/tier Dad").Kind != Unknown {
		t.Fatal("missing tier should be Unknown")
	}
}

func TestNoteCarriesText(t *testing.T) {
	cmd := Parse("today I'm at the clinic")
	if cmd.Kind != Note || cmd.Text != "today I'm at the clinic" {
		t.Fatalf("note: %+v", cmd)
	}
}
