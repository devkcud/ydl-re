package storage

import (
	"context"
	"fmt"
	"os"
	"testing"

	"yoinkdl/internal/ranges"
)

func TestResumeRefusesUnsafeMutablePartial(t *testing.T) {
	for _, version := range []int{ManifestVersion, RangeManifestVersion} {
		for _, mode := range []string{"hardlink", "shared-read", "shared-write"} {
			t.Run(fmt.Sprintf("v%d/%s", version, mode), func(t *testing.T) {
				path, f := securityPartial(t, version)
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}

				write(t, path+PartSuffix, "abcolddata")
				before := readBytes(t, path+ManifestSuffix)
				switch mode {
				case "hardlink":
					if err := os.Link(path+PartSuffix, path+".other"); err != nil {
						t.Fatal(err)
					}
				case "shared-read":
					if err := os.Chmod(path+PartSuffix, 0o644); err != nil {
						t.Fatal(err)
					}
				case "shared-write":
					if err := os.Chmod(path+PartSuffix, 0o620); err != nil {
						t.Fatal(err)
					}
				}

				saved, err := Open(context.Background(), path, false, true)
				if err == nil {
					err = appendSecurityPartial(saved, version)
					saved.Close()
				}

				if err == nil {
					t.Error("unsafe partial accepted mutable transfer bytes")
				}

				assertData(t, path+PartSuffix, "abcolddata")
				assertData(t, path+ManifestSuffix, string(before))
				if mode == "hardlink" {
					assertData(t, path+".other", "abcolddata")
				}
				assertAbsent(t, path)
			})
		}
	}
}

func TestActiveWritesRefuseNewPartialAliases(t *testing.T) {
	for _, version := range []int{ManifestVersion, RangeManifestVersion} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			path, f := securityPartial(t, version)
			if version == ManifestVersion {
				if err := f.Checkpoint(true); err != nil {
					t.Fatal(err)
				}
			}
			before := readBytes(t, path+ManifestSuffix)
			var w *RangeWriter
			var err error
			if version == RangeManifestVersion {
				w, err = f.ClaimRange(ranges.Span{Start: 3, End: 6})
				if err != nil {
					t.Fatal(err)
				}
			}

			if err := os.Link(path+PartSuffix, path+".other"); err != nil {
				t.Fatal(err)
			}

			var n int
			if w == nil {
				n, err = f.Write([]byte("def"))
			} else {
				n, err = w.Write([]byte("def"))
				if w.Commit(context.Background()) == nil {
					t.Fatal("failed writer advanced its checkpoint")
				}
				w.Close()
			}
			if n != 0 || err == nil {
				t.Fatalf("wrote through an alias: %d, %v", n, err)
			}

			if err := f.Finalize(); err == nil {
				t.Fatal("published an aliased mutable file")
			}

			assertData(t, path+".other", "abc")
			assertData(t, path+PartSuffix, "abc")
			assertData(t, path+ManifestSuffix, string(before))
			assertAbsent(t, path)
		})
	}
}

func TestSequentialPublicationChecksStoredBytes(t *testing.T) {
	for _, length := range []int64{-1, 6} {
		for _, data := range []string{"baddef", "abc", "abcdefextra"} {
			t.Run(fmt.Sprintf("%d/%s", length, data), func(t *testing.T) {
				path, f := startTransfer(t, length)
				writeBytes(t, f, "abcdef")
				before := readBytes(t, path+ManifestSuffix)
				write(t, path+PartSuffix, data)

				if err := f.Finalize(); err == nil {
					t.Fatal("published bytes that differ from the received stream")
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}

				assertAbsent(t, path)
				assertData(t, path+PartSuffix, data)
				assertData(t, path+ManifestSuffix, string(before))
			})
		}
	}
}

func appendSecurityPartial(f *File, version int) error {
	if version == ManifestVersion {
		if err := f.Continue(); err != nil {
			return err
		}

		_, err := f.Write([]byte("def"))
		return err
	}

	if err := f.ContinueRanges(); err != nil {
		return err
	}

	w, err := f.ClaimRange(ranges.Span{Start: 3, End: 6})
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = w.Write([]byte("def"))
	return err
}

func securityPartial(t *testing.T, version int) (string, *File) {
	t.Helper()
	if version == ManifestVersion {
		path, f := startTransfer(t, 10)
		writeBytes(t, f, "abc")
		return path, f
	}

	path, f := startRanges(t, 10, 3)
	putChunk(t, f, 0, "abc")
	return path, f
}
