package app

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInitialSnapshotNeedsNoDirectory(t *testing.T) {
	s := testSnapshot(t)
	p := s.Profiles[0]
	if s.Version != 1 || s.InstanceID != testID(1) || len(s.Profiles) != 1 ||
		p.ID != testID(2) || p.Revision != 1 || p.Name != "Default" || p.OutputDir != nil ||
		p.Defaults != (Defaults{Connections: 4, Retries: 3, TimeoutNS: 30000000000}) ||
		s.Submissions == nil || len(s.Submissions) != 0 {
		t.Fatal("initial snapshot did not preserve explicit identity and unset destination")
	}
	if _, err := os.Stat(s.StateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("constructor touched the state directory: %v", err)
	}
	if got, err := NewSnapshot("invalid", testID(2), s.StateDir); err != ErrInvalid || !reflect.DeepEqual(got, Snapshot{}) {
		t.Fatal("invalid initialization returned partial metadata")
	}
}

func TestSnapshotRejectsInvalidMetadata(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Snapshot)
	}{
		{"version", func(s *Snapshot) { s.Version++ }},
		{"instance", func(s *Snapshot) { s.InstanceID = strings.Repeat("A", 32) }},
		{"relative state", func(s *Snapshot) { s.StateDir = "relative" }},
		{"unclean state", func(s *Snapshot) { s.StateDir += "/../state" }},
		{"empty profiles", func(s *Snapshot) { s.Profiles = []Profile{} }},
		{"nil profiles", func(s *Snapshot) { s.Profiles = nil }},
		{"nil submissions", func(s *Snapshot) { s.Submissions = nil }},
		{"duplicate profile", func(s *Snapshot) { s.Profiles = append(s.Profiles, s.Profiles[0]) }},
		{"profile ID", func(s *Snapshot) { s.Profiles[0].ID = strings.Repeat("z", 32) }},
		{"revision", func(s *Snapshot) { s.Profiles[0].Revision = 0 }},
		{"blank name", func(s *Snapshot) { s.Profiles[0].Name = "" }},
		{"untrimmed name", func(s *Snapshot) { s.Profiles[0].Name = " Pato" }},
		{"control name", func(s *Snapshot) { s.Profiles[0].Name = "Pa\nto" }},
		{"invalid UTF8", func(s *Snapshot) { s.Profiles[0].Name = "\xff" }},
		{"name bytes", func(s *Snapshot) { s.Profiles[0].Name = strings.Repeat("é", 41) }},
		{"path bytes", func(s *Snapshot) { *s.Profiles[0].OutputDir += strings.Repeat("x", MaxPathSize) }},
		{"empty directory", func(s *Snapshot) { *s.Profiles[0].OutputDir = "" }},
		{"path NUL", func(s *Snapshot) { *s.Profiles[0].OutputDir += "\x00" }},
		{"path UTF8", func(s *Snapshot) { s.StateDir += "\xff" }},
		{"zero connections", func(s *Snapshot) { s.Profiles[0].Connections = 0 }},
		{"excess connections", func(s *Snapshot) { s.Profiles[0].Connections = 33 }},
		{"negative retries", func(s *Snapshot) { s.Profiles[0].Retries = -1 }},
		{"excess retries", func(s *Snapshot) { s.Profiles[0].Retries = 11 }},
		{"timeout", func(s *Snapshot) { s.Profiles[0].TimeoutNS = 0 }},
		{"limit", func(s *Snapshot) { s.Profiles[0].LimitBPS = -1 }},
		{"submission ID", func(s *Snapshot) { s.Submissions[0].ID = "short" }},
		{"duplicate submission", func(s *Snapshot) { s.Submissions = append(s.Submissions, s.Submissions[0]) }},
		{"unknown profile", func(s *Snapshot) { s.Submissions[0].ProfileID = testID(9) }},
		{"zero reference", func(s *Snapshot) { s.Submissions[0].ProfileRevision = 0 }},
		{"future reference", func(s *Snapshot) { s.Submissions[0].ProfileRevision++ }},
		{"unset destination", func(s *Snapshot) { s.Profiles[0].OutputDir = nil }},
		{"wrong directory", func(s *Snapshot) { s.Submissions[0].Settings.Output = filepath.Join(s.StateDir, "other", "file") }},
		{"reserved output", func(s *Snapshot) { s.Submissions[0].Settings.Output += ".yoinkdl.part" }},
		{"filename separator", func(s *Snapshot) { s.Submissions[0].Settings.Output += `\other` }},
		{"output root", func(s *Snapshot) { s.Submissions[0].Settings.Output = string(filepath.Separator) }},
		{"unresolved settings", func(s *Snapshot) { s.Submissions[0].Settings.Connections = 0 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testSubmitted(t)
			test.edit(&s)
			if err := s.Validate(); err != ErrInvalid {
				t.Fatalf("invalid metadata error = %v", err)
			}
		})
	}
}

func TestSnapshotAllowsHistoricalSettings(t *testing.T) {
	s := testSubmitted(t)
	s.Profiles[0].Revision = math.MaxInt64
	s.Profiles[0].OutputDir = nil
	s.Profiles[0].Name = "Pato 🐈"
	s.Profiles[0].Defaults = Defaults{Connections: 32, Retries: 10, TimeoutNS: math.MaxInt64, LimitBPS: math.MaxInt64}
	s.Submissions[0].Settings.Defaults = Defaults{Connections: 1, TimeoutNS: 1}
	s.Submissions[0].Settings.Resume = true
	if err := s.Validate(); err != nil {
		t.Fatal("historical settings were bound to the edited profile", err)
	}
}

func TestSnapshotCollectionBounds(t *testing.T) {
	s := testSubmitted(t)
	for i := 1; i < MaxProfiles; i++ {
		p := s.Profiles[0]
		p.ID = testID(i + 2)
		s.Profiles = append(s.Profiles, p)
	}
	for i := 1; i < MaxSubmissions; i++ {
		sub := s.Submissions[0]
		sub.ID = testID(i + 3)
		s.Submissions = append(s.Submissions, sub)
	}
	if err := s.Validate(); err != nil {
		t.Fatal("rejected full collections or duplicate display names/outputs", err)
	}
	p := s.Profiles[0]
	p.ID = testID(100)
	s.Profiles = append(s.Profiles, p)
	if err := s.Validate(); err != ErrInvalid {
		t.Fatal("accepted excess profiles")
	}
	s.Profiles = s.Profiles[:MaxProfiles]
	sub := s.Submissions[0]
	sub.ID = testID(1000)
	s.Submissions = append(s.Submissions, sub)
	if err := s.Validate(); err != ErrInvalid {
		t.Fatal("accepted excess submissions")
	}
}

func testSnapshot(t *testing.T) Snapshot {
	t.Helper()
	s, err := NewSnapshot(testID(1), testID(2), filepath.Join(t.TempDir(), "not-created"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func testSubmitted(t *testing.T) Snapshot {
	t.Helper()
	s := testSnapshot(t)
	dir := filepath.Join(s.StateDir, "downloads")
	s.Profiles[0].OutputDir = &dir
	s.Submissions = append(s.Submissions, Submission{
		ID: testID(3), ProfileID: s.Profiles[0].ID, ProfileRevision: 1,
		Settings: Settings{Output: filepath.Join(dir, "file"), Defaults: s.Profiles[0].Defaults},
	})
	return s
}

func testID(n int) string {
	return fmt.Sprintf("%032x", n)
}
