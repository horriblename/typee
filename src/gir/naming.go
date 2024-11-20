package gir

import (
	"strings"
	"unicode"
)

func snake_case_to_PascalCase(name string) string {
	return camelCaseInner(name, true)
}

func snake_case_to_camelCase(name string) string {
	return camelCaseInner(name, false)
}

func camelCaseInner(name string, capitalizeFirst bool) string {
	if name == "" {
		return ""
	}

	capitalize := capitalizeFirst
	var b strings.Builder
	for _, c := range name {
		if c == '_' {
			capitalize = true
			continue
		}

		if capitalize {
			b.WriteRune(unicode.ToUpper(c))
			capitalize = false
		} else {
			b.WriteRune(c)
		}
	}

	return b.String()
}

func ctorSuffix(name string) string {
	if len(name) > 4 {
		return snake_case_to_PascalCase(name)
	}
	return ""
}
