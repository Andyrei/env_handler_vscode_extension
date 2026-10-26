package runner

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMergeEnvFileWins(t *testing.T) {
	got := MergeEnv(
		[]string{"PATH=/bin", "KEEP=parent", "VALUE=parent"},
		map[string]string{"VALUE": "file", "NEW": "file"},
		true,
	)
	want := []string{"KEEP=parent", "NEW=file", "PATH=/bin", "VALUE=file"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("MergeEnv = %q, want %q", got, want)
	}
}

func TestMergeEnvParentWins(t *testing.T) {
	got := MergeEnv(
		[]string{"VALUE=parent"},
		map[string]string{"VALUE": "file", "NEW": "file"},
		false,
	)
	want := []string{"NEW=file", "VALUE=parent"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("MergeEnv = %q, want %q", got, want)
	}
}

func TestMergeEnvCaseInsensitive(t *testing.T) {
	got := mergeEnv(
		[]string{"Path=C:\\bin"},
		map[string]string{"PATH": "D:\\tools"},
		true,
		true,
	)
	want := []string{"PATH=D:\\tools"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("mergeEnv = %q, want %q", got, want)
	}
}

func TestRunReturnsChildExitCode(t *testing.T) {
	code, err := Run(Options{
		Args:   []string{os.Args[0], "-test.run=TestHelperProcess", "--", "exit"},
		Parent: append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HELPER_MODE=exit"),
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
}

func TestRunLoadsValues(t *testing.T) {
	var out bytes.Buffer
	code, err := Run(Options{
		Args:   []string{os.Args[0], "-test.run=TestHelperProcess", "--", "print"},
		Parent: append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HELPER_MODE=print", "HELPER_VALUE=parent"),
		Values: map[string]string{"HELPER_VALUE": "from-file"},
		Stdout: &out,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(out.String()) != "from-file" {
		t.Fatalf("stdout = %q, want from-file", out.String())
	}
}

func TestRunNoOverride(t *testing.T) {
	var out bytes.Buffer
	code, err := Run(Options{
		Args:       []string{os.Args[0], "-test.run=TestHelperProcess", "--", "print"},
		Parent:     append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "HELPER_MODE=print", "HELPER_VALUE=parent"),
		Values:     map[string]string{"HELPER_VALUE": "from-file"},
		NoOverride: true,
		Stdout:     &out,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(out.String()) != "parent" {
		t.Fatalf("stdout = %q, want parent", out.String())
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	switch os.Getenv("HELPER_MODE") {
	case "print":
		_, _ = os.Stdout.WriteString(os.Getenv("HELPER_VALUE") + "\n")
		os.Exit(0)
	case "exit":
		os.Exit(7)
	default:
		os.Exit(0)
	}
}
