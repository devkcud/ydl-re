package jsoncheck

import "testing"

func TestValidUnicode(t *testing.T) {
	for _, text := range []string{
		`{"name":"Pato 🐈"}`, `"\ud83d\ude00"`, `"\uD83D\uDE00"`,
		`"\\ud800"`, `"\ufffd"`, `"escaped \" quote"`, `"backslash \\"`,
		``, `not JSON`, `"\q"`,
	} {
		if !ValidUnicode([]byte(text)) {
			t.Fatalf("refused lossless text: %q", text)
		}
	}

	for _, text := range []string{
		"\xff", "\"\xff\"", `"\ud800"`, `"\udc00"`, `"\ud800\u0020"`,
		`"\udc00\ud800"`, `"\ud800\ud800"`, `"\uzzzz"`, `"\u12"`,
		`"\ud800x"`, `"\ud800\u"`, `"unterminated`, `"trailing\`,
	} {
		if ValidUnicode([]byte(text)) {
			t.Fatalf("accepted lossy or incomplete text: %q", text)
		}
	}
}
