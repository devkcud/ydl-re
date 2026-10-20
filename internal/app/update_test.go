package app

import (
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSnapshotCopiesOwnMutableData(t *testing.T) {
	s := testSubmitted(t)
	want := s.clone()
	next := s.clone()
	if !reflect.DeepEqual(next, s) {
		t.Fatal("copy changed metadata")
	}
	next.Profiles[0].Name = "Changed"
	*next.Profiles[0].OutputDir = filepath.Join(s.StateDir, "other")
	next.Submissions[0].Settings.Connections++
	if !reflect.DeepEqual(s, want) {
		t.Fatal("candidate shares mutable storage with the original snapshot")
	}
	initial := testSnapshot(t)
	if !reflect.DeepEqual(initial.clone(), initial) {
		t.Fatal("copy changed nullable directory or empty collection")
	}
}

func TestProfileEditsPreserveSubmittedSettings(t *testing.T) {
	s := testSubmitted(t)
	next := s.clone()
	next.Profiles[0].Revision++
	next.Profiles[0].Name = "Pato"
	next.Profiles[0].Connections = 8
	*next.Profiles[0].OutputDir = filepath.Join(s.StateDir, "other")
	if err := validateUpdate(s, next); err != nil || next.Submissions[0] != s.Submissions[0] {
		t.Fatal("profile edit changed or invalidated an existing submission", err)
	}

	p := next.Profiles[0]
	p.ID, p.Revision = testID(4), 1
	next.Profiles = append(next.Profiles, p)
	sub := next.Submissions[0]
	sub.ID, sub.ProfileRevision = testID(5), next.Profiles[0].Revision
	sub.Settings.Output = filepath.Join(*next.Profiles[0].OutputDir, "new-file")
	next.Submissions = append(next.Submissions, sub)
	if err := validateUpdate(s, next); err != nil {
		t.Fatal("rejected a new profile or current-revision submission", err)
	}
	s = next.clone()
	next.Submissions = next.Submissions[:0]
	if err := validateUpdate(s, next); err != nil {
		t.Fatal("explicit submission removal was refused", err)
	}
}

func TestSnapshotUpdateRejectsIdentityAndHistoryChanges(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*Snapshot)
	}{
		{"instance", func(s *Snapshot) { s.InstanceID = testID(9) }},
		{"state directory", func(s *Snapshot) { s.StateDir = filepath.Join(s.StateDir, "other") }},
		{"profile removed", func(s *Snapshot) { s.Profiles = s.Profiles[:1] }},
		{"profile ID", func(s *Snapshot) { s.Profiles[1].ID = testID(9) }},
		{"revision backward", func(s *Snapshot) { s.Profiles[0].Revision-- }},
		{"revision without edit", func(s *Snapshot) { s.Profiles[0].Revision++ }},
		{"unversioned edit", func(s *Snapshot) { s.Profiles[0].Name = "Changed" }},
		{"revision skipped", func(s *Snapshot) {
			s.Profiles[0].Name = "Changed"
			s.Profiles[0].Revision += 2
		}},
		{"reassigned submission", func(s *Snapshot) { s.Submissions[0].ProfileID = s.Profiles[1].ID }},
		{"submission revision", func(s *Snapshot) { s.Submissions[0].ProfileRevision++ }},
		{"submission output", func(s *Snapshot) { s.Submissions[0].Settings.Output += "-changed" }},
		{"submission settings", func(s *Snapshot) { s.Submissions[0].Settings.Connections++ }},
		{"submission resume", func(s *Snapshot) { s.Submissions[0].Settings.Resume = true }},
		{"stale new submission", func(s *Snapshot) {
			sub := s.Submissions[0]
			sub.ID = testID(5)
			s.Submissions = append(s.Submissions, sub)
		}},
		{"new profile revision", func(s *Snapshot) {
			p := s.Profiles[0]
			p.ID = testID(5)
			s.Profiles = append(s.Profiles, p)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testSubmitted(t)
			s.Profiles[0].Revision = 2
			p := s.Profiles[0]
			p.ID = testID(4)
			s.Profiles = append(s.Profiles, p)
			next := s.clone()
			test.edit(&next)
			if err := next.Validate(); err != nil {
				t.Fatal("fixture must be independently valid", err)
			}
			if err := validateUpdate(s, next); err != ErrConflict {
				t.Fatalf("update error = %v", err)
			}
		})
	}
}

func TestSnapshotUpdateRejectsInvalidStatesAndRevisionOverflow(t *testing.T) {
	s := testSubmitted(t)
	s.Profiles[0].Revision = math.MaxInt64
	next := s.clone()
	if err := validateUpdate(s, next); err != nil {
		t.Fatal("no-op update failed at the revision limit", err)
	}
	next.Profiles[0].Name = "Changed"
	if err := validateUpdate(s, next); err != ErrConflict {
		t.Fatal("profile revision exhaustion allowed an edit")
	}
	if validateUpdate(Snapshot{}, s) != ErrInvalid || validateUpdate(s, Snapshot{}) != ErrInvalid {
		t.Fatal("update bypassed snapshot validation")
	}
}
