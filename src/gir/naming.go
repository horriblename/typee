package gir

import (
	"strings"
	"unicode"
)

func snakeToPascalCase(name string) string {
	if name == "" {
		return ""
	}

	capitalize := true
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
		return snakeToPascalCase(name)
	}
	return ""
}
