package app

import (
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

const Version = 1
const MaxProfiles = 32
const MaxSubmissions = 512
const MaxNameSize = 80
const MaxPathSize = 4096
const MaxSnapshotSize = 4 * 1024 * 1024

var ErrInvalid = errors.New("invalid application metadata")

type Defaults struct {
	Connections int   `json:"connections"`
	Retries     int   `json:"retries"`
	TimeoutNS   int64 `json:"timeout_ns,string"`
	LimitBPS    int64 `json:"limit_bps,string"`
}

type Profile struct {
	ID        string  `json:"id"`
	Revision  int64   `json:"revision,string"`
	Name      string  `json:"name"`
	OutputDir *string `json:"output_dir"`
	Defaults
}

type Settings struct {
	Output string `json:"output"`
	Defaults
	Resume bool `json:"resume"`
}

type Submission struct {
	ID              string   `json:"id"`
	ProfileID       string   `json:"profile_id"`
	ProfileRevision int64    `json:"profile_revision,string"`
	Settings        Settings `json:"settings"`
}

type Snapshot struct {
	Version     int          `json:"version"`
	InstanceID  string       `json:"instance_id"`
	StateDir    string       `json:"state_dir"`
	Profiles    []Profile    `json:"profiles"`
	Submissions []Submission `json:"submissions"`
}

func NewSnapshot(id, profile, dir string) (Snapshot, error) {
	s := Snapshot{
		Version: Version, InstanceID: id, StateDir: dir,
		Profiles: []Profile{{
			ID: profile, Revision: 1, Name: "Default",
			Defaults: Defaults{
				Connections: transfer.DefaultConnections, Retries: transfer.DefaultRetries,
				TimeoutNS: int64(transfer.DefaultTimeout),
			},
		}},
		Submissions: []Submission{},
	}
	if err := s.Validate(); err != nil {
		return Snapshot{}, err
	}

	return s, nil
}

func (s Snapshot) Validate() error {
	_, err := encodeSnapshot(s)
	return err
}

func (s Snapshot) validate() error {
	if s.Version != Version || !validID(s.InstanceID) || !validPath(s.StateDir) ||
		len(s.Profiles) == 0 || len(s.Profiles) > MaxProfiles ||
		s.Submissions == nil || len(s.Submissions) > MaxSubmissions {
		return ErrInvalid
	}

	profiles := make(map[string]Profile, len(s.Profiles))
	for _, p := range s.Profiles {
		if !p.valid() || profiles[p.ID].ID != "" {
			return ErrInvalid
		}
		profiles[p.ID] = p
	}

	ids := make(map[string]bool, len(s.Submissions))
	for _, sub := range s.Submissions {
		p, found := profiles[sub.ProfileID]
		if !validID(sub.ID) || ids[sub.ID] || !found ||
			sub.ProfileRevision <= 0 || sub.ProfileRevision > p.Revision || !sub.Settings.valid() {
			return ErrInvalid
		}
		if sub.ProfileRevision == p.Revision &&
			(p.OutputDir == nil || filepath.Dir(sub.Settings.Output) != *p.OutputDir) {
			return ErrInvalid
		}
		ids[sub.ID] = true
	}

	return nil
}

func (p Profile) valid() bool {
	return validID(p.ID) && p.Revision > 0 && p.Defaults.valid() &&
		p.Name != "" && len(p.Name) <= MaxNameSize && utf8.ValidString(p.Name) &&
		strings.TrimSpace(p.Name) == p.Name && strings.IndexFunc(p.Name, unicode.IsControl) < 0 &&
		(p.OutputDir == nil || validPath(*p.OutputDir))
}

func (d Defaults) valid() bool {
	return d.Connections >= 1 && d.Connections <= transfer.MaxConnections &&
		d.Retries >= 0 && d.Retries <= transfer.MaxRetries && d.TimeoutNS > 0 && d.LimitBPS >= 0
}

func (s Settings) valid() bool {
	return s.Defaults.valid() && validPath(s.Output) && filepath.Dir(s.Output) != s.Output &&
		!strings.ContainsRune(filepath.Base(s.Output), '\\') && !storage.Reserved(s.Output)
}

func validID(id string) bool {
	if len(id) != 32 || strings.ToLower(id) != id {
		return false
	}

	_, err := hex.DecodeString(id)
	return err == nil
}

func validPath(path string) bool {
	return len(path) <= MaxPathSize && utf8.ValidString(path) && !strings.ContainsRune(path, 0) &&
		filepath.IsAbs(path) && filepath.Clean(path) == path
}
