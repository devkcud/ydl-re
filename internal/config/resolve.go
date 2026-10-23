package config

import (
	"errors"
	"path/filepath"
	"slices"
	"time"

	"yoinkdl/internal/transfer"
)

var ErrScope = errors.New("select global or one configuration profile")
var ErrUnset = errors.New("configuration setting is unset")

type Source string

const SourceBuiltin Source = "builtin"
const SourceGlobal Source = "global"
const SourceProfile Source = "profile"

type Scope struct {
	Global  bool
	Profile string
}

type Entry struct {
	Key    Key
	Value  Value
	Source Source
}

type Settings struct {
	OutputDir   string
	Connections int
	Retries     int
	TimeoutNS   int64
	LimitBPS    int64
	StateDir    string
	StagingDir  string
}

type Resolution struct {
	ProfileID   string
	ProfileName string
	Settings    Settings
	Entries     []Entry
}

func Defaults(dir string) (Resolution, error) {
	if !validPath(dir) || !validPath(filepath.Join(dir, "transfers")) {
		return Resolution{}, ErrInvalid
	}

	return resolve(dir, nil, Profile{})
}

func (c Config) Resolve(scope Scope) (Resolution, error) {
	if err := c.Validate(); err != nil {
		return Resolution{}, err
	}

	i, err := c.scopeIndex(scope)
	if err != nil {
		return Resolution{}, err
	}

	var profile Profile
	if i >= 0 {
		profile = c.Profiles[i]
	}

	return resolve(c.StateDir, c.Defaults, profile)
}

func (c Config) Get(scope Scope, key Key) (Value, error) {
	if !slices.Contains(keys[:], key) {
		return Value{}, ErrKey
	}

	values, _, err := c.values(scope)
	if err != nil {
		return Value{}, err
	}

	value, found := values[key]
	if !found {
		return Value{}, ErrUnset
	}

	return value, nil
}

func (c Config) List(scope Scope) ([]Entry, error) {
	values, source, err := c.values(scope)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(values))
	for _, key := range keys {
		if value, found := values[key]; found {
			entries = append(entries, Entry{Key: key, Value: value, Source: source})
		}
	}

	return entries, nil
}

func (c Config) scopeIndex(scope Scope) (int, error) {
	if scope.Global {
		if scope.Profile != "" {
			return 0, ErrScope
		}
		return -1, nil
	}

	if scope.Profile == "" {
		return 0, ErrScope
	}

	return c.profileIndex(scope.Profile)
}

func (c Config) values(scope Scope) (Overrides, Source, error) {
	if err := c.Validate(); err != nil {
		return nil, "", err
	}

	i, err := c.scopeIndex(scope)
	if err != nil {
		return nil, "", err
	}
	if i < 0 {
		return c.Defaults, SourceGlobal, nil
	}

	return c.Profiles[i].Settings, SourceProfile, nil
}

func resolve(dir string, defaults Overrides, profile Profile) (Resolution, error) {
	values := Overrides{
		Connections: Int(transfer.DefaultConnections),
		Limit:       Text(Unlimited),
		OutputDir:   {},
		Retries:     Int(transfer.DefaultRetries),
		Timeout:     Text(transfer.DefaultTimeout.String()),
		StagingDir:  Text(Destination),
		StateDir:    Text(filepath.Join(dir, "transfers")),
	}
	r := Resolution{
		ProfileID: profile.ID, ProfileName: profile.Name,
		Entries: make([]Entry, 0, len(keys)),
	}
	for _, key := range keys {
		source := SourceBuiltin
		if value, found := defaults[key]; found {
			values[key] = value
			source = SourceGlobal
		}
		if value, found := profile.Settings[key]; found {
			values[key] = value
			source = SourceProfile
		}

		r.Entries = append(r.Entries, Entry{Key: key, Value: values[key], Source: source})
	}

	timeout, err := time.ParseDuration(values[Timeout].text)
	if err != nil {
		return Resolution{}, ErrInvalid
	}

	var limit int64
	if values[Limit].text != Unlimited {
		limit, err = transfer.ParseLimit(values[Limit].text)
		if err != nil {
			return Resolution{}, ErrInvalid
		}
	}

	r.Settings = Settings{
		OutputDir: values[OutputDir].text, Connections: values[Connections].num,
		Retries: values[Retries].num, TimeoutNS: int64(timeout), LimitBPS: limit,
		StateDir: values[StateDir].text, StagingDir: values[StagingDir].text,
	}
	return r, nil
}
