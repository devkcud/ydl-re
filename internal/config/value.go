package config

import (
	"errors"
	"slices"
	"time"

	"yoinkdl/internal/transfer"
)

type Key string

const Connections Key = "download.connections"
const Limit Key = "download.limit"
const OutputDir Key = "download.output_dir"
const Retries Key = "download.retries"
const Timeout Key = "download.timeout"
const StagingDir Key = "storage.staging_dir"
const StateDir Key = "storage.state_dir"

const Unlimited = "unlimited"
const Destination = "destination"

var ErrKey = errors.New("unknown configuration setting")
var ErrValue = errors.New("invalid configuration value")

var keys = [...]Key{Connections, Limit, OutputDir, Retries, Timeout, StagingDir, StateDir}

type valueKind uint8

const (
	unsetValue valueKind = iota
	intValue
	textValue
)

type Value struct {
	kind valueKind
	num  int
	text string
}

func Int(n int) Value {
	return Value{kind: intValue, num: n}
}

func Text(s string) Value {
	return Value{kind: textValue, text: s}
}

func (v Value) Int() (int, bool) {
	return v.num, v.kind == intValue
}

func (v Value) Text() (string, bool) {
	return v.text, v.kind == textValue
}

func Keys() []Key {
	return slices.Clone(keys[:])
}

func (k Key) validate(v Value) error {
	if !slices.Contains(keys[:], k) {
		return ErrKey
	}

	valid := false
	switch k {
	case Connections:
		valid = v.kind == intValue && v.num >= 1 && v.num <= transfer.MaxConnections
	case Retries:
		valid = v.kind == intValue && v.num >= 0 && v.num <= transfer.MaxRetries
	case OutputDir, StateDir:
		valid = v.kind == textValue && validPath(v.text)
	case StagingDir:
		valid = v.kind == textValue && (v.text == Destination || validPath(v.text))
	case Timeout:
		n, err := time.ParseDuration(v.text)
		valid = v.kind == textValue && err == nil && n > 0
	case Limit:
		_, err := transfer.ParseLimit(v.text)
		valid = v.kind == textValue && (v.text == Unlimited || err == nil)
	}

	if !valid {
		return ErrValue
	}

	return nil
}
