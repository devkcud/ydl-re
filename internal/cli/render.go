package cli

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func safeText(value string) string {
	if !utf8.ValidString(value) || strings.IndexFunc(value, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.Quote(value)
	}
	return value
}
