package app

import (
	"errors"
	"math"
	"slices"
)

var ErrConflict = errors.New("application metadata conflicts with committed state")

func validateUpdate(before, after Snapshot) error {
	if err := before.Validate(); err != nil {
		return err
	}
	if err := after.Validate(); err != nil {
		return err
	}
	if before.InstanceID != after.InstanceID || before.StateDir != after.StateDir {
		return ErrConflict
	}

	profiles := make(map[string]Profile, len(before.Profiles))
	for _, p := range before.Profiles {
		profiles[p.ID] = p
	}
	revisions := make(map[string]int64, len(after.Profiles))
	for _, p := range after.Profiles {
		revision := int64(1)
		if old, found := profiles[p.ID]; found {
			revision = old.Revision
			if !sameProfile(old, p) {
				if revision == math.MaxInt64 {
					return ErrConflict
				}
				revision++
			}
			delete(profiles, p.ID)
		}
		if p.Revision != revision {
			return ErrConflict
		}
		revisions[p.ID] = revision
	}
	if len(profiles) != 0 {
		return ErrConflict
	}

	submissions := make(map[string]Submission, len(before.Submissions))
	for _, sub := range before.Submissions {
		submissions[sub.ID] = sub
	}
	for _, sub := range after.Submissions {
		old, found := submissions[sub.ID]
		if found && old != sub || !found && sub.ProfileRevision != revisions[sub.ProfileID] {
			return ErrConflict
		}
	}

	return nil
}

func sameProfile(a, b Profile) bool {
	return a.Name == b.Name && a.Defaults == b.Defaults &&
		(a.OutputDir == nil && b.OutputDir == nil ||
			a.OutputDir != nil && b.OutputDir != nil && *a.OutputDir == *b.OutputDir)
}

func (s Snapshot) clone() Snapshot {
	s.Profiles = slices.Clone(s.Profiles)
	for i := range s.Profiles {
		if s.Profiles[i].OutputDir != nil {
			dir := *s.Profiles[i].OutputDir
			s.Profiles[i].OutputDir = &dir
		}
	}
	s.Submissions = slices.Clone(s.Submissions)
	return s
}
