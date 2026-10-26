package shell

import "testing"

func TestValidName(t *testing.T) {
	valid := []string{"PATH", "_PRIVATE", "A1", "lower_case"}
	for _, name := range valid {
		if !ValidName(name) {
			t.Errorf("ValidName(%q) = false, want true", name)
		}
	}
	invalid := []string{"", "1ABC", "A-B", "A.B", "A B", "A=B"}
	for _, name := range invalid {
		if ValidName(name) {
			t.Errorf("ValidName(%q) = true, want false", name)
		}
	}
}

func TestQuote(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"", "''"},
		{"plain", "'plain'"},
		{"two words", "'two words'"},
		{"it's", `'it'\''s'`},
		{"line\nbreak", "'line\nbreak'"},
		{"$HOME `id`\";", "'$HOME `id`\";'"},
	}
	for _, test := range tests {
		if got := Quote(test.value); got != test.want {
			t.Errorf("Quote(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestExport(t *testing.T) {
	if got, want := Export("TOKEN", "a b'c"), `export TOKEN='a b'\''c'`; got != want {
		t.Fatalf("Export = %q, want %q", got, want)
	}
}
