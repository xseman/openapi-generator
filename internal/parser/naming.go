package parser

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// unsafeCommentChars maps comment delimiters in spec description text to
// harmless variants so they cannot terminate a generated doc comment early
// (e.g. a Kubernetes description containing "'*/scale'"), mirroring upstream
// AbstractTypeScriptClientCodegen.escapeUnsafeCharacters.
var unsafeCommentChars = strings.NewReplacer("*/", "*_/", "/*", "/_*")

func escapeUnsafeChars(s string) string {
	return unsafeCommentChars.Replace(s)
}

func toCamelCase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return s
	}
	titleCaser := cases.Title(language.English)
	result := strings.ToLower(words[0])
	for _, word := range words[1:] {
		result += titleCaser.String(strings.ToLower(word))
	}
	return result
}

func toPascalCase(s string) string {
	words := splitWords(s)
	titleCaser := cases.Title(language.English)
	result := ""
	for _, word := range words {
		result += titleCaser.String(strings.ToLower(word))
	}
	return result
}

func toSnakeCase(s string) string {
	words := splitWords(s)
	result := ""
	for i, word := range words {
		if i > 0 {
			result += "_"
		}
		result += strings.ToLower(word)
	}
	return result
}

// splitWords splits s on non-alphanumeric characters and camelCase boundaries.
func splitWords(s string) []string {
	var words []string
	var current strings.Builder

	for i, r := range s {
		if !isAlphanumeric(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}

		if i > 0 && isUpperCase(r) && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isUpperCase(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

// sanitizeTag converts a URL path into a valid PascalCase identifier, used to
// synthesize an operation ID when the spec doesn't provide one.
func sanitizeTag(path string) string {
	// Convert path to a valid identifier
	path = strings.ReplaceAll(path, "/", "_")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.ReplaceAll(path, "-", "_")
	return toPascalCase(path)
}

// toEnumVarName builds the member name (key) for a generated enum entry. TypeScript-fetch
// uses PascalCase member names (matching upstream openapi-generator's default
// ENUM_PROPERTY_NAMING of PascalCase), while the caller preserves the underlying string
// value verbatim — so CALL_CENTRUM yields the key CallCentrum mapped to 'CALL_CENTRUM'.
//
// It replicates camelize(underscore(value)): a separator is inserted at camelCase
// boundaries, the whole string is lower-cased so SCREAMING_SNAKE_CASE runs collapse into
// single words (the camelCase splitter would otherwise fragment them into individual
// letters, e.g. ODPOCTAR -> O,D,P,... -> ODPOCTAR), then each word is Title-cased.
func toEnumVarName(value string) string {
	// Spell out "+" so values such as "C+" survive the alphanumeric-only word split.
	value = strings.ReplaceAll(value, "+", "_plus")

	var b strings.Builder
	var prev rune
	for i, r := range value {
		if i > 0 && isUpperCase(r) && ((prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')) {
			b.WriteByte('_')
		}
		b.WriteRune(r)
		prev = r
	}

	name := toPascalCase(strings.ToLower(b.String()))
	if name == "" {
		return "Empty"
	}
	// Ensure the result is a valid identifier start.
	if name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}
	return name
}
