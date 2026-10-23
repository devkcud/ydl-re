package config

import (
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestKeyCatalogAndValueTypes(t *testing.T) {
	want := []Key{Connections, Limit, OutputDir, Retries, Timeout, StagingDir, StateDir}
	got := Keys()
	if !reflect.DeepEqual(got, want) {
		t.Fatal("key catalog is incomplete or out of order")
	}
	got[0] = "other"
	if !reflect.DeepEqual(Keys(), want) {
		t.Fatal("caller changed the key catalog")
	}

	if n, ok := Int(0).Int(); !ok || n != 0 {
		t.Fatal("explicit zero lost its type")
	}
	if s, ok := Text(Unlimited).Text(); !ok || s != Unlimited {
		t.Fatal("text value lost its type")
	}
	if _, ok := Text("4").Int(); ok {
		t.Fatal("text was coerced into an integer")
	}
	if _, ok := Int(4).Text(); ok {
		t.Fatal("integer was coerced into text")
	}
	if _, ok := (Value{}).Text(); ok {
		t.Fatal("unset value became explicit text")
	}
}

func TestSettingValuesUseExistingBoundsAndGrammar(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created")
	max := strconv.FormatInt(math.MaxInt64, 10)
	for _, test := range []struct {
		key   Key
		valid []Value
		bad   []Value
	}{
		{Connections, []Value{Int(1), Int(32)}, []Value{Int(0), Int(-1), Int(33), Text("4")}},
		{Retries, []Value{Int(0), Int(10)}, []Value{Int(-1), Int(11), Text("3")}},
		{Timeout, []Value{Text("1ns"), Text("1m30s"), Text(max + "ns")}, []Value{Text("0s"), Text("-1s"), Text("1"), Text(max + "s"), Int(30)}},
		{Limit, []Value{Text(Unlimited), Text("01B/s"), Text("5MB/s"), Text("5MiB/s"), Text(max + "B/s")}, []Value{Text("0B/s"), Text("-1B/s"), Text("1.5MB/s"), Text("5mb/s"), Text(" 1B/s"), Text(max + "KB/s"), Text("Unlimited"), Int(0)}},
		{OutputDir, []Value{Text(dir)}, []Value{Text(""), Text("relative"), Text(dir + "/../out"), Text(dir + "\x00"), Text(dir + "\xff"), Text(dir + strings.Repeat("x", MaxPathSize)), Int(1)}},
		{StateDir, []Value{Text(dir)}, []Value{Text(""), Text("relative"), Text(Unlimited), Int(1)}},
		{StagingDir, []Value{Text(dir), Text(Destination)}, []Value{Text(""), Text("relative"), Text("Destination"), Int(1)}},
	} {
		t.Run(string(test.key), func(t *testing.T) {
			for _, value := range test.valid {
				if err := test.key.validate(value); err != nil {
					t.Fatal("rejected valid setting", err)
				}
			}
			for _, value := range append(test.bad, Value{}) {
				if err := test.key.validate(value); err != ErrValue {
					t.Fatal("accepted invalid or unset explicit setting", err)
				}
			}
		})
	}

	if Key("secret=invalid").validate(Text("private")) != ErrKey {
		t.Fatal("unknown setting did not return a fixed error")
	}
}
