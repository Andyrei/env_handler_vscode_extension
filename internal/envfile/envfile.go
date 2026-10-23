package envfile

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrEncrypted is returned when an edit targets a read-only encrypted entry.
	ErrEncrypted = errors.New("encrypted values are read-only")
	// ErrInvalidKey is returned when a key cannot be written safely.
	ErrInvalidKey = errors.New("invalid environment key")
)

// Entry describes one variable assignment in a .env file.
type Entry struct {
	Key       string
	Value     string
	Raw       string
	Quote     byte
	KeyStart  int
	KeyEnd    int
	Start     int
	End       int
	LineStart int
	LineEnd   int
	Line      int
	Index     int
	Encrypted bool
	Multiline bool
}

// Parse reads .env text into entries while preserving source offsets.
func Parse(text string) ([]Entry, error) {
	entries := make([]Entry, 0)
	pos := 0
	line := 1

	if strings.HasPrefix(text, "\uFEFF") {
		pos = len("\uFEFF")
	}

	for pos < len(text) {
		lineStart := pos
		lineEnd := strings.IndexByte(text[pos:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += pos
		}

		contentEnd := lineEnd
		if contentEnd > pos && text[contentEnd-1] == '\r' {
			contentEnd--
		}

		p := pos
		for p < contentEnd && isHorizontalSpace(text[p]) {
			p++
		}
		if p >= contentEnd || text[p] == '#' {
			next := nextLine(text, lineEnd)
			line += strings.Count(text[lineStart:next], "\n")
			pos = next
			continue
		}

		if hasExportPrefix(text, p, contentEnd) {
			p += len("export")
			for p < contentEnd && isHorizontalSpace(text[p]) {
				p++
			}
		}

		keyStart := p
		for p < contentEnd && !isKeyDelimiter(text[p]) {
			p++
		}
		if p == keyStart {
			next := nextLine(text, lineEnd)
			line += strings.Count(text[lineStart:next], "\n")
			pos = next
			continue
		}
		keyEnd := p
		key := text[keyStart:keyEnd]

		for p < contentEnd && isHorizontalSpace(text[p]) {
			p++
		}
		if p >= contentEnd || (text[p] != '=' && text[p] != ':') {
			next := nextLine(text, lineEnd)
			line += strings.Count(text[lineStart:next], "\n")
			pos = next
			continue
		}
		p++
		for p < contentEnd && isHorizontalSpace(text[p]) {
			p++
		}

		entry := Entry{
			Key:       key,
			KeyStart:  keyStart,
			KeyEnd:    keyEnd,
			LineStart: lineStart,
			Line:      line,
			Index:     len(entries),
		}

		if p >= contentEnd || (contentEnd == lineEnd && p == lineEnd) {
			entry.Start = p
			entry.End = p
			entry.Raw = ""
			entry.Value = ""
			entry.LineEnd = nextLine(text, lineEnd)
			entries = append(entries, entry)
			next := nextLine(text, lineEnd)
			line += strings.Count(text[lineStart:next], "\n")
			pos = next
			continue
		}

		switch text[p] {
		case '"', '\'', '`':
			quote := text[p]
			closeEnd, err := findClosingQuote(text, p, quote)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			raw := text[p:closeEnd]
			entry.Quote = quote
			entry.Start = p
			entry.End = closeEnd
			entry.Raw = raw
			entry.Value = decodeQuoted(raw, quote)
			entry.Multiline = strings.Contains(raw, "\n")
			entry.Encrypted = strings.HasPrefix(entry.Value, "encrypted:")
			entry.LineEnd = nextLine(text, closeEnd)
			entries = append(entries, entry)
			line += strings.Count(text[lineStart:entry.LineEnd], "\n")
			pos = entry.LineEnd
			continue
		default:
			valueEnd := unquotedValueEnd(text, p, contentEnd)
			raw := text[p:valueEnd]
			entry.Start = p
			entry.End = valueEnd
			entry.Raw = raw
			entry.Value = raw
			entry.Encrypted = strings.HasPrefix(entry.Value, "encrypted:")
			entry.LineEnd = nextLine(text, lineEnd)
			entries = append(entries, entry)
			line += strings.Count(text[lineStart:entry.LineEnd], "\n")
			pos = entry.LineEnd
			continue
		}
	}

	return entries, nil
}

// SetValue replaces an entry value while preserving its quote style.
func SetValue(text string, entry Entry, value string) (string, error) {
	if entry.Encrypted {
		return "", ErrEncrypted
	}
	replacement, err := formatValue(value, entry.Quote)
	if err != nil {
		return "", err
	}
	return text[:entry.Start] + replacement + text[entry.End:], nil
}

// RenameKey replaces an entry key while preserving export/spacing.
func RenameKey(text string, entry Entry, key string) (string, error) {
	if entry.Encrypted {
		return "", ErrEncrypted
	}
	if err := validateKey(key); err != nil {
		return "", err
	}
	return text[:entry.KeyStart] + key + text[entry.KeyEnd:], nil
}

// RemoveEntry removes the entry's whole line, including its newline.
func RemoveEntry(text string, entry Entry) (string, error) {
	if entry.Encrypted {
		return "", ErrEncrypted
	}
	return text[:entry.LineStart] + text[entry.LineEnd:], nil
}

// AddKey appends a new assignment with a trailing newline.
func AddKey(text, key, value string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}
	formatted, err := formatValue(value, 0)
	if err != nil {
		return "", err
	}
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	line := key + "=" + formatted + eol
	if text == "" {
		return line, nil
	}
	if strings.HasSuffix(text, "\n") {
		return text + line, nil
	}
	return text + eol + line, nil
}

func hasExportPrefix(text string, start, end int) bool {
	const word = "export"
	if start+len(word) > end {
		return false
	}
	if text[start:start+len(word)] != word {
		return false
	}
	return start+len(word) < end && isHorizontalSpace(text[start+len(word)])
}

func findClosingQuote(text string, start int, quote byte) (int, error) {
	for i := start + 1; i < len(text); i++ {
		if text[i] != quote {
			continue
		}
		if quote == '"' && isEscaped(text, start, i) {
			continue
		}
		return i + 1, nil
	}
	return 0, errors.New("unterminated quoted value")
}

func isEscaped(text string, start, index int) bool {
	backslashes := 0
	for i := index - 1; i > start && text[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func decodeQuoted(raw string, quote byte) string {
	if len(raw) < 2 {
		return raw
	}
	value := raw[1 : len(raw)-1]
	if quote != '"' {
		return value
	}

	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			b.WriteByte('\\')
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

func unquotedValueEnd(text string, start, end int) int {
	if start >= end {
		return start
	}
	if text[start] == '#' {
		return start
	}

	valueEnd := end
	for i := start + 1; i < end; i++ {
		if text[i] == '#' && isHorizontalSpace(text[i-1]) {
			valueEnd = i
			break
		}
	}
	for valueEnd > start && isHorizontalSpace(text[valueEnd-1]) {
		valueEnd--
	}
	return valueEnd
}

func formatValue(value string, quote byte) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return doubleQuote(value), nil
	}

	switch quote {
	case '\'':
		if !strings.ContainsRune(value, '\'') {
			return "'" + value + "'", nil
		}
		return doubleQuote(value), nil
	case '`':
		if !strings.ContainsRune(value, '`') {
			return "`" + value + "`", nil
		}
		return doubleQuote(value), nil
	case '"':
		return doubleQuote(value), nil
	default:
		if isSafeUnquoted(value) {
			return value, nil
		}
		return doubleQuote(value), nil
	}
}

func doubleQuote(value string) string {
	var b strings.Builder
	b.Grow(len(value) + 2)
	b.WriteByte('"')
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(value[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}

func isSafeUnquoted(value string) bool {
	if value == "" {
		return true
	}
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case ' ', '\t', '\r', '\n', '#', '\'', '"', '`':
			return false
		}
	}
	return true
}

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty key", ErrInvalidKey)
	}
	for i := 0; i < len(key); i++ {
		switch key[i] {
		case ' ', '\t', '\r', '\n', '=', ':', '#':
			return fmt.Errorf("%w: %q", ErrInvalidKey, key)
		}
	}
	return nil
}

func isHorizontalSpace(ch byte) bool {
	return ch == ' ' || ch == '\t'
}

func isKeyDelimiter(ch byte) bool {
	return isHorizontalSpace(ch) || ch == '=' || ch == ':'
}

func nextLine(text string, from int) int {
	index := strings.IndexByte(text[from:], '\n')
	if index < 0 {
		return len(text)
	}
	return from + index + 1
}
