package postman

import (
	"errors"
	"strings"
	"testing"
)

func TestParseObjectAndArray(t *testing.T) {
	single := []byte(`{"name":"Staging","values":[{"key":"API","value":"https://x","enabled":true}]}`)
	envs, err := Parse(single)
	if err != nil || len(envs) != 1 || envs[0].Name != "Staging" {
		t.Fatalf("unexpected parse of object: %v %+v", err, envs)
	}

	list := []byte(`[{"name":"A","values":[]},{"name":"B","values":[]}]`)
	envs, err = Parse(list)
	if err != nil || len(envs) != 2 {
		t.Fatalf("unexpected parse of array: %v %+v", err, envs)
	}
}

func TestParseRejectsNonEnvironments(t *testing.T) {
	for _, input := range []string{``, `{"info":{"name":"collection"}}`, `[]`, `not json`} {
		if _, err := Parse([]byte(input)); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid for %q, got %v", input, err)
		}
	}
}

func TestParseErrorDoesNotEchoContent(t *testing.T) {
	_, err := Parse([]byte(`{"values":"supersecret`))
	if err == nil || strings.Contains(err.Error(), "supersecret") {
		t.Fatalf("error must not echo file content: %v", err)
	}
}

func TestRenderEnabledDisabledAndSecrets(t *testing.T) {
	yes, no := true, false
	env := Environment{
		Name: "Staging",
		Values: []Variable{
			{Key: "API_URL", Value: "https://api.example.test", Enabled: &yes},
			{Key: "OLD_URL", Value: "https://old.example.test", Enabled: &no},
			{Key: "TOKEN", Value: "abc def", Type: "secret"},
			{Key: "BAD KEY", Value: "x"},
			{Key: "COUNT", Value: 3.0},
		},
	}

	out := Render(env, false)
	want := "API_URL=https://api.example.test\n" +
		"# OLD_URL=https://old.example.test\n" +
		"TOKEN=\"abc def\"\n" +
		"COUNT=3\n"
	if out.Text != want {
		t.Fatalf("unexpected text:\n%s\nwant:\n%s", out.Text, want)
	}
	if len(out.SkippedKeys) != 1 || out.SkippedKeys[0] != "BAD KEY" {
		t.Fatalf("expected BAD KEY to be skipped, got %v", out.SkippedKeys)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("expected a secret warning, got %v", out.Warnings)
	}

	skipped := Render(env, true)
	if strings.Contains(skipped.Text, "OLD_URL") {
		t.Fatal("disabled values should be omitted with skipDisabled")
	}
}
