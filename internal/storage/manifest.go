package storage

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

const ManifestVersion = 1
const RangeManifestVersion = 2
const StatePartial = "partial"
const StateReady = "ready"
const maxManifestSize = 64 * 1024

type Manifest struct {
	Version     int     `json:"schema_version"`
	Destination string  `json:"destination"`
	SourceID    string  `json:"source_id"`
	EffectiveID string  `json:"effective_id"`
	Length      int64   `json:"length"`
	ETag        string  `json:"etag"`
	Encoding    string  `json:"encoding"`
	Completed   int64   `json:"completed"`
	Checksum    string  `json:"sha256"`
	State       string  `json:"state"`
	ChunkSize   int64   `json:"-"`
	Ranges      []Chunk `json:"-"`
}

func (m Manifest) clone() Manifest {
	m.Ranges = slices.Clone(m.Ranges)
	return m
}

func (m Manifest) Resumable() bool {
	return m.State == StateReady || (m.Length >= 0 && m.Encoding == "identity" && StrongETag(m.ETag))
}

func StrongETag(tag string) bool {
	if len(tag) < 2 || len(tag) > 4096 || tag[0] != '"' || tag[len(tag)-1] != '"' || !utf8.ValidString(tag) {
		return false
	}

	for _, b := range []byte(tag[1 : len(tag)-1]) {
		if b < 0x21 || b == 0x22 || b == 0x7f {
			return false
		}
	}

	return true
}

func Inspect(path string) (Manifest, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Manifest{}, err
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return Manifest{}, err
	}
	defer root.Close()

	return readManifest(root, filepath.Base(path)+ManifestSuffix, path)
}

func (m Manifest) validate(path string) error {
	if (m.Version != ManifestVersion && m.Version != RangeManifestVersion) || m.Destination != path || !filepath.IsAbs(m.Destination) {
		return errors.New("unsupported manifest version or mismatched destination")
	}

	if !validHash(m.SourceID) || !validHash(m.EffectiveID) {
		return errors.New("invalid manifest identity or checksum")
	}

	if m.ETag != "" && !StrongETag(m.ETag) {
		return errors.New("invalid stored ETag")
	}

	if m.Encoding != "identity" && m.Encoding != "encoded" {
		return errors.New("invalid stored encoding")
	}

	if m.Version == RangeManifestVersion {
		return m.validateRanges()
	}

	if !validHash(m.Checksum) || m.ChunkSize != 0 || m.Ranges != nil {
		return errors.New("invalid prefix checksum or incompatible range fields")
	}

	if m.Length < -1 || m.Completed < 0 || (m.Length >= 0 && m.Completed > m.Length) {
		return errors.New("invalid checkpoint bounds")
	}

	switch m.State {
	case StatePartial:
		if m.Length > 0 && m.Completed == m.Length {
			return errors.New("partial checkpoint cannot assert complete coverage")
		}
	case StateReady:
		if m.Length < 0 || m.Completed != m.Length {
			return errors.New("ready checkpoint must cover the whole file")
		}
	default:
		return errors.New("invalid checkpoint state")
	}

	if m.Completed == 0 && m.Checksum != fmt.Sprintf("%x", sha256.Sum256(nil)) {
		return errors.New("invalid empty-prefix checksum")
	}

	return nil
}

func readManifest(root *os.Root, name, path string) (Manifest, error) {
	file, err := openRegular(root, name, os.O_RDONLY)
	if err != nil {
		return Manifest{}, fmt.Errorf("open transfer manifest: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxManifestSize+1))
	if err != nil || len(data) > maxManifestSize {
		return Manifest{}, errors.New("unreadable or oversized transfer manifest")
	}

	m, err := decodeManifest(data)
	if err != nil {
		return Manifest{}, err
	}

	if err := m.validate(path); err != nil {
		return Manifest{}, err
	}

	return m, nil
}

func (f *File) saveManifest(m Manifest) error {
	if err := m.validate(f.path); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil || len(data) >= maxManifestSize {
		return errors.New("cannot encode transfer manifest")
	}

	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}

	name := f.name + TempPrefix + hex.EncodeToString(token[:])
	temp, err := f.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create manifest checkpoint: %w", err)
	}
	defer f.root.Remove(name)

	_, writeErr := temp.Write(append(data, '\n'))
	err = errors.Join(writeErr, temp.Sync(), temp.Close())
	if err != nil {
		return fmt.Errorf("write manifest checkpoint: %w", err)
	}

	if f.manifest == nil {
		err = f.root.Link(name, f.name+ManifestSuffix)
	} else {
		err = f.root.Rename(name, f.name+ManifestSuffix)
	}

	if err != nil {
		return fmt.Errorf("publish manifest checkpoint: %w", err)
	}

	saved := m.clone()
	f.manifest = &saved
	return f.syncDirectory()
}

func validHash(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}

	_, err := hex.DecodeString(value)
	return err == nil
}

func objectFields(data []byte) (map[string]json.RawMessage, error) {
	invalid := errors.New("JSON fields must be non-null, unique, and correctly named")
	if !utf8.Valid(data) {
		return nil, invalid
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return nil, invalid
	}

	fields := make(map[string]json.RawMessage)

	for dec.More() {
		token, err := dec.Token()
		key, ok := token.(string)
		if err != nil || !ok || fields[key] != nil {
			return nil, invalid
		}

		var value json.RawMessage

		if err := dec.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, invalid
		}

		fields[key] = value
	}

	if token, err := dec.Token(); err != nil || token != json.Delim('}') {
		return nil, invalid
	}

	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, invalid
	}

	return fields, nil
}

func hasFields(fields map[string]json.RawMessage, names ...string) bool {
	if len(fields) != len(names) {
		return false
	}

	for _, name := range names {
		if fields[name] == nil {
			return false
		}
	}

	return true
}
