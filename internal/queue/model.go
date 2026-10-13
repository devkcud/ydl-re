package queue

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"yoinkdl/internal/storage"
	"yoinkdl/internal/transfer"
)

const Version = 1
const DefaultParallel = 1
const MaxParallel = 8
const MaxJobs = 256
const MaxURLSize = 16 * 1024
const MaxPathSize = 4096
const MaxErrorSize = 4096
const MaxSnapshotSize = 8 * 1024 * 1024

type State string

const (
	Queued      State = "queued"
	Running     State = "running"
	Pausing     State = "pausing"
	Paused      State = "paused"
	Canceling   State = "canceling"
	Canceled    State = "canceled"
	Interrupted State = "interrupted"
	Failed      State = "failed"
	Completed   State = "completed"
)

type Request struct {
	URL         *string `json:"url"`
	Output      string  `json:"output"`
	Connections int     `json:"connections"`
	Retries     int     `json:"retries"`
	TimeoutNS   int64   `json:"timeout_ns"`
	LimitBPS    int64   `json:"limit_bps"`
	Resume      bool    `json:"resume"`
}

type Job struct {
	ID        string  `json:"id"`
	Order     int64   `json:"order"`
	Request   Request `json:"request"`
	InputHash string  `json:"input_hash"`
	State     State   `json:"state"`
	Published *bool   `json:"published"`
	Error     *string `json:"error"`
}

type Snapshot struct {
	Version   int   `json:"version"`
	Parallel  int   `json:"parallel"`
	NextOrder int64 `json:"next_order"`
	Jobs      []Job `json:"jobs"`
}

func NewSnapshot() Snapshot {
	return Snapshot{Version: Version, Parallel: DefaultParallel, NextOrder: 1, Jobs: []Job{}}
}

func NewJob(id string, order int64, req transfer.Request) (Job, error) {
	if req.Overwrite {
		return Job{}, errors.New("queue requests cannot overwrite output")
	}

	if !utf8.ValidString(req.URL) || len(req.URL) > MaxURLSize {
		return Job{}, errors.New("invalid queue source URL")
	}

	req, err := transfer.Normalize(req)
	if err != nil {
		return Job{}, err
	}

	source := req.URL
	r := Request{
		URL: &source, Output: req.Output, Connections: req.Connections, Retries: *req.Retries,
		TimeoutNS: int64(req.Timeout), LimitBPS: req.Limit, Resume: req.Resume,
	}

	published := false
	j := Job{ID: id, Order: order, Request: r, State: Queued, Published: &published}
	j.InputHash, err = r.hash()
	if err != nil {
		return Job{}, err
	}

	if err := j.validate(); err != nil {
		return Job{}, err
	}

	return j, nil
}

func (r Request) Transfer() (transfer.Request, error) {
	if r.URL == nil {
		return transfer.Request{}, errors.New("queue source is no longer retained")
	}

	return r.validate()
}

func (r Request) validate() (transfer.Request, error) {
	if !utf8.ValidString(r.Output) || len(r.Output) > MaxPathSize || strings.ContainsRune(r.Output, 0) ||
		!filepath.IsAbs(r.Output) || filepath.Clean(r.Output) != r.Output || storage.Reserved(r.Output) {
		return transfer.Request{}, errors.New("queue output must be an absolute clean path outside reserved artifact names")
	}

	if r.Connections <= 0 || r.TimeoutNS <= 0 {
		return transfer.Request{}, errors.New("queue request must contain resolved transfer settings")
	}

	source := "https://queue.invalid/"
	if r.URL != nil {
		if len(*r.URL) > MaxURLSize || !utf8.ValidString(*r.URL) {
			return transfer.Request{}, errors.New("invalid queue source URL")
		}

		source = *r.URL
	}

	retries := r.Retries
	req, err := transfer.Normalize(transfer.Request{
		URL: source, Output: r.Output, Connections: r.Connections, Retries: &retries,
		Timeout: time.Duration(r.TimeoutNS), Limit: r.LimitBPS, Resume: r.Resume,
	})
	if err != nil {
		return transfer.Request{}, err
	}

	if req.URL != source {
		return transfer.Request{}, errors.New("queue source URL must be normalized")
	}

	return req, nil
}

func (r Request) hash() (string, error) {
	if _, err := r.Transfer(); err != nil {
		return "", err
	}

	data, err := json.Marshal(r)
	if err != nil {
		return "", errors.New("cannot encode queue request")
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (j Job) validate() error {
	if !validHex(j.ID, 16) || j.Order <= 0 || j.Order == math.MaxInt64 {
		return errors.New("invalid queue job identity or order")
	}

	if !validHex(j.InputHash, sha256.Size) {
		return errors.New("invalid queue input hash")
	}

	if _, err := j.Request.validate(); err != nil {
		return err
	}

	terminal := j.State == Completed || j.State == Canceled
	if terminal != (j.Request.URL == nil) {
		return errors.New("queue source retention does not match job state")
	}

	if j.Request.URL != nil {
		sum, err := j.Request.hash()
		if err != nil || sum != j.InputHash {
			return errors.New("queue request does not match its input hash")
		}
	}

	if j.Error != nil && (!utf8.ValidString(*j.Error) || len(*j.Error) > MaxErrorSize || strings.TrimSpace(*j.Error) == "") {
		return errors.New("invalid queue error description")
	}

	switch j.State {
	case Queued, Paused, Canceled:
		if j.Error != nil {
			return errors.New("queue state cannot carry an error")
		}
	case Running, Pausing, Canceling:
		if j.Published != nil || j.Error != nil {
			return errors.New("active queue attempt must have an unknown outcome")
		}
	case Interrupted:
		if j.Published != nil || j.Error == nil {
			return errors.New("interrupted queue attempt requires an unknown outcome and error")
		}
	case Failed:
		if j.Published == nil || j.Error == nil {
			return errors.New("failed queue attempt requires a result and error")
		}
	case Completed:
		if j.Published == nil || !*j.Published || j.Error != nil {
			return errors.New("completed queue job requires successful publication")
		}
	default:
		return errors.New("invalid queue job state")
	}

	return nil
}

func (s Snapshot) Validate() error {
	_, err := encodeSnapshot(s)
	return err
}

func (s Snapshot) validate() error {
	if s.Version != Version || s.Parallel < 1 || s.Parallel > MaxParallel || s.NextOrder < 1 {
		return errors.New("unsupported queue version or invalid settings")
	}

	if s.Jobs == nil || len(s.Jobs) > MaxJobs {
		return errors.New("queue jobs must be an array of at most 256 records")
	}

	ids := make(map[string]bool, len(s.Jobs))
	orders := make(map[int64]bool, len(s.Jobs))
	paths := make(map[string]bool, len(s.Jobs))

	for _, j := range s.Jobs {
		if err := j.validate(); err != nil {
			return err
		}

		if j.Order >= s.NextOrder || ids[j.ID] || orders[j.Order] || paths[j.Request.Output] {
			return errors.New("duplicate queue identity, order, output, or invalid next order")
		}

		ids[j.ID] = true
		orders[j.Order] = true
		paths[j.Request.Output] = true
	}

	return nil
}

func (s Snapshot) clone() Snapshot {
	s.Jobs = slices.Clone(s.Jobs)
	for i := range s.Jobs {
		s.Jobs[i].Request.URL = copyPtr(s.Jobs[i].Request.URL)
		s.Jobs[i].Published = copyPtr(s.Jobs[i].Published)
		s.Jobs[i].Error = copyPtr(s.Jobs[i].Error)
	}

	return s
}

func validHex(value string, size int) bool {
	if len(value) != size*2 || strings.ToLower(value) != value {
		return false
	}

	_, err := hex.DecodeString(value)
	return err == nil
}

func copyPtr[T any](value *T) *T {
	if value == nil {
		return nil
	}

	copy := *value
	return &copy
}
