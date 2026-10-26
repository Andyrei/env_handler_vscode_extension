// Package runner executes a command with an environment loaded from env files.
package runner

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
)

// Options describes one child process.
type Options struct {
	Dir    string
	Args   []string
	Values map[string]string
	// NoOverride lets the parent process win when a key exists in both places.
	// The default gives the env file values precedence.
	NoOverride bool
	Parent     []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

// Run starts the command, forwards interrupt signals, and returns its exit code.
// A non-zero exit code from the child is not an error.
func Run(opts Options) (int, error) {
	if len(opts.Args) == 0 {
		return 0, errors.New("no command")
	}

	parent := opts.Parent
	if parent == nil {
		parent = os.Environ()
	}

	cmd := exec.Command(opts.Args[0], opts.Args[1:]...)
	cmd.Dir = opts.Dir
	cmd.Env = MergeEnv(parent, opts.Values, !opts.NoOverride)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, forwardedSignals()...)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-signals:
				if cmd.Process != nil {
					_ = cmd.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()

	err := cmd.Wait()
	close(done)
	signal.Stop(signals)

	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitCodeFor(exitErr.ProcessState), nil
	}
	return 0, err
}

// MergeEnv combines the parent environment and file values.
// When fileWins is true, file values override the parent.
func MergeEnv(parent []string, values map[string]string, fileWins bool) []string {
	return mergeEnv(parent, values, fileWins, runtime.GOOS == "windows")
}

type envEntry struct {
	name  string
	value string
}

func mergeEnv(parent []string, values map[string]string, fileWins, caseInsensitive bool) []string {
	index := make(map[string]int, len(parent)+len(values))
	merged := make([]envEntry, 0, len(parent)+len(values))

	key := func(name string) string {
		if caseInsensitive {
			return strings.ToUpper(name)
		}
		return name
	}
	set := func(name, value string) {
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return
		}
		k := key(name)
		if at, ok := index[k]; ok {
			merged[at] = envEntry{name: name, value: value}
			return
		}
		index[k] = len(merged)
		merged = append(merged, envEntry{name: name, value: value})
	}

	if !fileWins {
		for name, value := range values {
			set(name, value)
		}
	}
	for _, item := range parent {
		name, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		set(name, value)
	}
	if fileWins {
		for name, value := range values {
			set(name, value)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return key(merged[i].name) < key(merged[j].name)
	})

	out := make([]string, 0, len(merged))
	for _, item := range merged {
		out = append(out, item.name+"="+item.value)
	}
	return out
}
