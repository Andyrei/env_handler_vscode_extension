// Package postman converts Postman environment exports into .env text.
package postman

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"env-handler/internal/envfile"
)

// ErrInvalid means the input is not a Postman environment export.
var ErrInvalid = errors.New("not a Postman environment export")

// Variable is one entry in a Postman environment.
type Variable struct {
	Key     string `json:"key"`
	Value   any    `json:"value"`
	Enabled *bool  `json:"enabled"`
	Type    string `json:"type"`
}

// Environment is one Postman environment export.
type Environment struct {
	Name   string     `json:"name"`
	Values []Variable `json:"values"`
}

// Rendered is the .env text for one environment plus what was left out.
type Rendered struct {
	Text        string
	SkippedKeys []string
	Warnings    []string
}

// Parse accepts a single environment object or an array of them.
// Error messages never include the file content.
func Parse(data []byte) ([]Environment, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, ErrInvalid
	}

	var envs []Environment
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &envs); err != nil {
			return nil, fmt.Errorf("%w: could not parse JSON", ErrInvalid)
		}
	} else {
		var env Environment
		if err := json.Unmarshal(trimmed, &env); err != nil {
			return nil, fmt.Errorf("%w: could not parse JSON", ErrInvalid)
		}
		envs = []Environment{env}
	}

	if len(envs) == 0 {
		return nil, ErrInvalid
	}
	for _, env := range envs {
		if env.Values == nil {
			return nil, fmt.Errorf("%w: missing values list", ErrInvalid)
		}
	}
	return envs, nil
}

// Render converts one environment to .env text. Disabled values become comments
// unless skipDisabled is set. Keys that cannot be written safely are reported.
func Render(env Environment, skipDisabled bool) Rendered {
	var out Rendered
	var b strings.Builder
	secrets := 0

	for _, variable := range env.Values {
		enabled := variable.Enabled == nil || *variable.Enabled
		if !enabled && skipDisabled {
			continue
		}
		if variable.Type == "secret" {
			secrets++
		}

		line, err := envfile.Line(strings.TrimSpace(variable.Key), stringify(variable.Value))
		if err != nil {
			out.SkippedKeys = append(out.SkippedKeys, variable.Key)
			continue
		}
		if !enabled {
			line = "# " + line
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	out.Text = b.String()
	if secrets > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("%d Postman secret value(s) are stored as plaintext.", secrets))
	}
	return out
}

func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
