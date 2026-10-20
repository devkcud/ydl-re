package app

import (
	"strconv"
	"unicode/utf8"
)

func validJSONText(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}

	quoted := false
	for i := 0; i < len(data); i++ {
		if data[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || data[i] != '\\' {
			continue
		}

		i++
		if i >= len(data) {
			return false
		}
		if data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return false
		}
		r, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil || r >= 0xdc00 && r <= 0xdfff {
			return false
		}
		i += 4
		if r < 0xd800 || r > 0xdbff {
			continue
		}
		if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return !quoted
}
