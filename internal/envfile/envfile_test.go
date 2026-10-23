package envfile

import (
	"errors"
	"strings"
	"testing"
)

func TestParseBasics(t *testing.T) {
	text := "# comment\nA=1\nexport B = \"two\"\nC: three\nD='four'\nE=`five`\n"

	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	assertEntry(t, entries[0], "A", "1", 0, 2)
	assertEntry(t, entries[1], "B", "two", '"', 3)
	assertEntry(t, entries[2], "C", "three", 0, 4)
	assertEntry(t, entries[3], "D", "four", '\'', 5)
	assertEntry(t, entries[4], "E", "five", '`', 6)

	if entries[1].KeyStart != strings.Index(text, "B") {
		t.Fatalf("expected exported key offset")
	}
}

func TestParseInlineCommentsAndEmptyValues(t *testing.T) {
	text := "A=one # keep\nB=#empty\nC=value#hash\nD=\"x\" # comment\n"

	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	assertEntry(t, entries[0], "A", "one", 0, 1)
	assertEntry(t, entries[1], "B", "", 0, 2)
	assertEntry(t, entries[2], "C", "value#hash", 0, 3)
	assertEntry(t, entries[3], "D", "x", '"', 4)

	updated, err := SetValue(text, entries[0], "new value")
	if err != nil {
		t.Fatalf("SetValue returned error: %v", err)
	}
	if !strings.Contains(updated, "A=\"new value\" # keep") {
		t.Fatalf("inline comment was not preserved: %q", updated)
	}

	updated, err = SetValue(text, entries[3], "new")
	if err != nil {
		t.Fatalf("SetValue returned error: %v", err)
	}
	if !strings.Contains(updated, "D=\"new\" # comment") {
		t.Fatalf("quoted inline comment was not preserved: %q", updated)
	}
}

func TestParseCRLFAndBOM(t *testing.T) {
	text := "\uFEFFA=1\r\nB=\"two\"\r\n"

	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	assertEntry(t, entries[0], "A", "1", 0, 1)
	assertEntry(t, entries[1], "B", "two", '"', 2)

	updated, err := AddKey(text, "C", "three")
	if err != nil {
		t.Fatalf("AddKey returned error: %v", err)
	}
	if !strings.HasSuffix(updated, "C=three\r\n") {
		t.Fatalf("expected CRLF append, got %q", updated)
	}
	if strings.Contains(strings.ReplaceAll(updated, "\r\n", ""), "\n") {
		t.Fatalf("mixed line ending introduced: %q", updated)
	}
}

func TestParseEscapes(t *testing.T) {
	text := "A=\"line\\nnext\\t\\\"q\\\"\\\\\"\n"

	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	want := "line\nnext\t\"q\"\\"
	if entries[0].Value != want {
		t.Fatalf("expected value %q, got %q", want, entries[0].Value)
	}
}

func TestParseMultiline(t *testing.T) {
	text := "A=\"one\ntwo\"\nB=3\n"

	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Value != "one\ntwo" {
		t.Fatalf("unexpected multiline value: %q", entries[0].Value)
	}
	if !entries[0].Multiline {
		t.Fatal("expected multiline entry")
	}
	if entries[1].Line != 3 {
		t.Fatalf("expected B on line 3, got %d", entries[1].Line)
	}

	updated, err := SetValue(text, entries[0], "single")
	if err != nil {
		t.Fatalf("SetValue returned error: %v", err)
	}
	if updated != "A=\"single\"\nB=3\n" {
		t.Fatalf("unexpected updated text: %q", updated)
	}

	removed, err := RemoveEntry(text, entries[0])
	if err != nil {
		t.Fatalf("RemoveEntry returned error: %v", err)
	}
	if removed != "B=3\n" {
		t.Fatalf("unexpected removed text: %q", removed)
	}
}

func TestDuplicatesAndEncrypted(t *testing.T) {
	text := "KEY=one\nKEY=two\nTOKEN=encrypted:abc\n"

	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Key != entries[1].Key {
		t.Fatal("expected duplicate keys to be preserved")
	}
	if entries[0].Index != 0 || entries[1].Index != 1 || entries[2].Index != 2 {
		t.Fatal("unexpected entry indexes")
	}
	if !entries[2].Encrypted {
		t.Fatal("expected encrypted entry")
	}

	if _, err := SetValue(text, entries[2], "plain"); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("expected ErrEncrypted, got %v", err)
	}
	if _, err := RenameKey(text, entries[2], "OTHER"); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("expected ErrEncrypted, got %v", err)
	}
	if _, err := RemoveEntry(text, entries[2]); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("expected ErrEncrypted, got %v", err)
	}
}

func TestFormattingEdits(t *testing.T) {
	text := "export A = old # keep\n"
	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	updated, err := SetValue(text, entries[0], "new value")
	if err != nil {
		t.Fatalf("SetValue returned error: %v", err)
	}
	if updated != "export A = \"new value\" # keep\n" {
		t.Fatalf("unexpected updated text: %q", updated)
	}

	renamed, err := RenameKey(updated, entries[0], "B")
	if err != nil {
		t.Fatalf("RenameKey returned error: %v", err)
	}
	if renamed != "export B = \"new value\" # keep\n" {
		t.Fatalf("unexpected renamed text: %q", renamed)
	}

	invalid, err := RenameKey(text, entries[0], "BAD KEY")
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
	if invalid != "" {
		t.Fatalf("expected empty result on error, got %q", invalid)
	}
}

func TestSingleQuoteFallback(t *testing.T) {
	text := "A='old'\n"
	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	updated, err := SetValue(text, entries[0], "it's")
	if err != nil {
		t.Fatalf("SetValue returned error: %v", err)
	}
	if updated != "A=\"it's\"\n" {
		t.Fatalf("unexpected updated text: %q", updated)
	}
}

func TestAddAndRemove(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty", "", "A=1\n"},
		{"trailing newline", "B=2\n", "B=2\nA=1\n"},
		{"no trailing newline", "B=2", "B=2\nA=1\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := AddKey(test.text, "A", "1")
			if err != nil {
				t.Fatalf("AddKey returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}

	text := "A=1\nB=2\nC=3\n"
	entries, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	removed, err := RemoveEntry(text, entries[1])
	if err != nil {
		t.Fatalf("RemoveEntry returned error: %v", err)
	}
	if removed != "A=1\nC=3\n" {
		t.Fatalf("unexpected removed text: %q", removed)
	}

	removed, err = RemoveEntry(text, entries[2])
	if err != nil {
		t.Fatalf("RemoveEntry returned error: %v", err)
	}
	if removed != "A=1\nB=2\n" {
		t.Fatalf("unexpected removed text: %q", removed)
	}
}

func TestParseSkipsInvalidLinesAndRejectsUnterminatedQuotes(t *testing.T) {
	entries, err := Parse("NOT_A_VAR\nA=1\n")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Key != "A" {
		t.Fatalf("unexpected entries: %#v", entries)
	}

	_, err = Parse("A=\"unterminated\n")
	if err == nil {
		t.Fatal("expected unterminated quote error")
	}
	if !strings.Contains(err.Error(), "unterminated quoted value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertEntry(t *testing.T, entry Entry, key, value string, quote byte, line int) {
	t.Helper()

	if entry.Key != key {
		t.Fatalf("expected key %q, got %q", key, entry.Key)
	}
	if entry.Value != value {
		t.Fatalf("expected value %q, got %q", value, entry.Value)
	}
	if entry.Quote != quote {
		t.Fatalf("expected quote %q, got %q", quote, entry.Quote)
	}
	if entry.Line != line {
		t.Fatalf("expected line %d, got %d", line, entry.Line)
	}
}
