package transfer

import (
	"net/url"
	"path"
	"strings"
	"unicode"

	"yoinkdl/internal/storage"
)

func filename(u *url.URL) string {
	if u.Path == "" || strings.HasSuffix(u.Path, "/") {
		return "download"
	}

	name, err := url.PathUnescape(path.Base(u.EscapedPath()))
	if err != nil {
		return "download"
	}

	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\<>:"|?*`, r) {
			return '_'
		}

		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" || len(name) > 200 {
		return "download"
	}

	base, _, _ := strings.Cut(strings.ToUpper(name), ".")
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
		(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
		name = "_" + name
	}

	if strings.HasSuffix(name, storage.PartSuffix) || strings.HasSuffix(name, storage.ManifestSuffix) {
		name += ".download"
	}

	return name
}
