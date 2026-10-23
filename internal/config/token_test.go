package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestConfigTokensDescribeExactValidatedBytes(t *testing.T) {
	c := testConfig(t)
	data, canonical, err := Encode(c)
	if err != nil || canonical != fmt.Sprintf("%x", sha256.Sum256(data)) {
		t.Fatal("encoded token does not describe returned bytes", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		t.Fatal(err)
	}
	variants := [][]byte{
		data,
		[]byte(" \t\n" + string(data) + " \t"),
		compact.Bytes(),
		[]byte(strings.Replace(string(data), `"Default"`, `"\u0044efault"`, 1)),
	}
	seen := make(map[string]bool)
	for _, raw := range variants {
		got, token, err := Decode(raw)
		if err != nil || !reflect.DeepEqual(got, c) || token != fmt.Sprintf("%x", sha256.Sum256(raw)) {
			t.Fatal("decoding changed config or hashed normalized bytes", err)
		}
		if seen[token] {
			t.Fatal("formatting/escaping edits did not change the token")
		}
		seen[token] = true
		encoded, token, err := Encode(got)
		if err != nil || !bytes.Equal(encoded, data) || token != canonical {
			t.Fatal("canonical representation was not stable", err)
		}
	}
}

func TestConfigTokensFollowEditsAndOwnTheirResult(t *testing.T) {
	c := codecConfig(t)
	data, original, err := Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	same, changed, err := c.Set(Scope{Global: true}, Connections, Int(4))
	if err != nil || changed {
		t.Fatal("fixture edit must be a no-op", err)
	}
	if next, token, err := Encode(same); err != nil || token != original || !bytes.Equal(next, data) {
		t.Fatal("canonical no-op changed bytes or token", err)
	}
	next, changed, err := c.Set(Scope{Global: true}, Connections, Int(8))
	if err != nil || !changed {
		t.Fatal(err)
	}
	updated, token, err := Encode(next)
	if err != nil || token == original || bytes.Equal(updated, data) {
		t.Fatal("global edit did not invalidate its content token", err)
	}
	got, loaded, err := Decode(data)
	if err != nil || loaded != original {
		t.Fatal(err)
	}
	clear(data)
	if loaded != original || !reflect.DeepEqual(got, c) {
		t.Fatal("caller bytes aliased decoded config or token")
	}
}

func TestInvalidConfigNeverExposesTokens(t *testing.T) {
	for _, data := range [][]byte{
		nil, []byte("null"), []byte(`{"version":1,"version":1}`),
		[]byte("\xff"), bytes.Repeat([]byte{' '}, MaxConfigSize+1),
	} {
		got, token, err := Decode(data)
		if err != ErrInvalid || token != "" || !reflect.DeepEqual(got, Config{}) {
			t.Fatal("invalid input exposed config or token", err)
		}
	}
	large := testConfig(t)
	large.Defaults = Overrides{Timeout: Text(strings.Repeat("0", MaxConfigSize*2) + "1s")}
	for _, c := range []Config{{}, large} {
		data, token, err := Encode(c)
		if err != ErrInvalid || data != nil || token != "" {
			t.Fatal("invalid encoding exposed bytes or token", err)
		}
	}
}
