package main

import (
	"regexp"
	"slices"
	"strings"
)

// keyChanges lists, by name only, how one dotenv file differs from another. OtherLines counts the lines that are
// neither KEY=VALUE nor part of a quoted value added or removed; their text is never kept, since it can be secret.
type keyChanges struct {
	Added      []string
	Removed    []string
	Changed    []string
	OtherLines int
}

func (c keyChanges) empty() bool {
	return len(c.Added) == 0 && len(c.Removed) == 0 && len(c.Changed) == 0 && c.OtherLines == 0
}

// envKey matches the upper-case shell variable names dotenv files use. Only such names are ever printed: an unquoted
// base64 line such as `c2VjcmV0=` is also a valid shell assignment, and its "name" is secret material, so anything
// with a lower-case letter is counted instead. A base64 fragment made only of upper-case letters and digits would
// still pass; for 16 characters that is about 1 in 10,000.
var envKey = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

type parsedEnv struct {
	vars  map[string]string
	other map[string]int
}

// parseEnv reads KEY=VALUE lines, skipping blank lines and # comments and allowing an `export` prefix. A value that
// starts with a quote it does not close takes the following lines up to the closing quote, whether or not its key is
// valid: a multi-line value such as a PEM key belongs to its key, and none of its lines is ever read as a key of its
// own. A quote inside an unquoted value (PW=it's) opens nothing, so the lines after it stay separate keys. Only
// leading whitespace is dropped; anything else that differs counts as a change. A key set twice keeps its last value,
// as sourcing the file would. Any other line is only counted, by content.
func parseEnv(data []byte) parsedEnv {
	env := parsedEnv{vars: map[string]string{}, other: map[string]int{}}
	lines := strings.Split(string(data), "\n")
	for i := 0; i < len(lines); i++ {
		block := strings.TrimLeft(strings.TrimRight(lines[i], "\r\n"), " \t")
		if strings.TrimSpace(block) == "" || strings.HasPrefix(block, "#") {
			continue
		}
		if quote := openingQuote(block); quote != 0 {
			for i+1 < len(lines) {
				i++
				next := strings.TrimRight(lines[i], "\r\n")
				block += "\n" + next
				if closesQuote(next, quote) {
					break
				}
			}
		}

		key, value, ok := strings.Cut(trimExport(block), "=")
		if key = strings.TrimRight(key, " \t"); ok && envKey.MatchString(key) {
			env.vars[key] = value
		} else {
			env.other[block]++
		}
	}

	return env
}

// openingQuote returns the quote a line's value (the text after the first =) starts with and leaves open at the end
// of the line, or 0. Whatever follows the closing quote never reopens one.
func openingQuote(line string) byte {
	_, value, ok := strings.Cut(line, "=")
	value = strings.TrimLeft(value, " \t")
	if !ok || value == "" || (value[0] != '"' && value[0] != '\'') || closesQuote(value[1:], value[0]) {
		return 0
	}

	return value[0]
}

// closesQuote reports whether s holds quote unescaped. Only double quotes honour backslash escapes, as in the shell.
func closesQuote(s string, quote byte) bool {
	for i := 0; i < len(s); i++ {
		switch {
		case quote == '"' && s[i] == '\\':
			i++
		case s[i] == quote:
			return true
		}
	}

	return false
}

func trimExport(s string) string {
	if rest, ok := strings.CutPrefix(s, "export"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
		return strings.TrimLeft(rest, " \t")
	}

	return s
}

func diffEnv(before, after []byte) keyChanges {
	old, cur := parseEnv(before), parseEnv(after)
	var c keyChanges
	for key, value := range cur.vars {
		prev, ok := old.vars[key]
		switch {
		case !ok:
			c.Added = append(c.Added, key)
		case prev != value:
			c.Changed = append(c.Changed, key)
		}
	}
	for key := range old.vars {
		if _, ok := cur.vars[key]; !ok {
			c.Removed = append(c.Removed, key)
		}
	}
	slices.Sort(c.Added)
	slices.Sort(c.Removed)
	slices.Sort(c.Changed)

	for line, n := range cur.other {
		c.OtherLines += max(n-old.other[line], 0)
	}
	for line, n := range old.other {
		c.OtherLines += max(n-cur.other[line], 0)
	}

	return c
}
