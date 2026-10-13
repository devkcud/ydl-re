package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

const Version = 1
const MaxRequestSize = 64 * 1024
const MaxReplySize = 8 * 1024 * 1024
const MaxClients = 16

var ErrProtocol = errors.New("invalid daemon protocol message")
var ErrState = errors.New("daemon state directory does not match")
var ErrConnection = errors.New("daemon connection failed; check explicit startup and selected paths")

type Status struct {
	queue.Status
	LimitBPS int64 `json:"limit_bps"`
}

type List struct {
	Daemon Status          `json:"daemon"`
	Jobs   []queue.JobView `json:"jobs"`
}

type Forgotten struct {
	ID        string `json:"id"`
	Forgotten bool   `json:"forgotten"`
}

type Reply struct {
	Version int             `json:"version"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result"`
	Error   *Failure        `json:"error"`
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (f *Failure) Error() string {
	return f.Message
}

type command struct {
	op       string
	id       string
	req      transfer.Request
	parallel int
}

func decodeCommand(data []byte, state string) (command, error) {
	var cmd command
	if len(data) > MaxRequestSize {
		return cmd, ErrProtocol
	}
	fields, err := object(data, "version", "state_dir", "op", "args")
	if err != nil || !nonNull(fields, "version", "state_dir", "op", "args") {
		return cmd, ErrProtocol
	}
	var version int
	var dir string
	if json.Unmarshal(fields["version"], &version) != nil || version != Version ||
		json.Unmarshal(fields["state_dir"], &dir) != nil || json.Unmarshal(fields["op"], &cmd.op) != nil ||
		!validPath(dir) {
		return cmd, ErrProtocol
	}
	if dir != state {
		return cmd, ErrState
	}
	var args map[string]json.RawMessage
	switch cmd.op {
	case "ping", "list", "stop":
		args, err = object(fields["args"])
	case "pause", "resume", "cancel", "forget":
		args, err = object(fields["args"], "id")
	case "set_parallel":
		args, err = object(fields["args"], "parallel")
	case "add":
		args, err = object(fields["args"], "id", "request")
	default:
		return cmd, ErrProtocol
	}
	if err != nil {
		return cmd, ErrProtocol
	}
	for name, data := range args {
		if isNull(data) {
			return cmd, ErrProtocol
		}
		switch name {
		case "id":
			if json.Unmarshal(data, &cmd.id) != nil || !validID(cmd.id) {
				return cmd, ErrProtocol
			}
		case "parallel":
			if json.Unmarshal(data, &cmd.parallel) != nil || cmd.parallel < 1 || cmd.parallel > queue.MaxParallel {
				return cmd, ErrProtocol
			}
		case "request":
			keys := []string{"url", "output", "connections", "retries", "timeout_ns", "limit_bps", "resume"}
			raw, err := object(data, keys...)
			var req queue.Request
			if err != nil || !nonNull(raw, keys...) || json.Unmarshal(data, &req) != nil {
				return cmd, ErrProtocol
			}
			cmd.req, err = req.Transfer()
			if err != nil {
				return cmd, ErrProtocol
			}
		}
	}
	return cmd, nil
}

func encodeReply(result any, err error) []byte {
	r := Reply{Version: Version, OK: err == nil}
	if err == nil {
		r.Result, err = json.Marshal(result)
		if err != nil || isNull(r.Result) {
			err = ErrProtocol
			r.OK, r.Result = false, nil
		}
	}
	if err != nil {
		r.Error = failure(err)
	}
	data, err := json.Marshal(r)
	if err != nil || len(data) >= MaxReplySize {
		return encodeReply(nil, ErrProtocol)
	}
	return append(data, '\n')
}

func decodeReply(data []byte, op string) (Reply, error) {
	var r Reply
	if len(data) > MaxReplySize {
		return r, ErrProtocol
	}
	fields, err := object(data, "version", "ok", "result", "error")
	if err != nil || !nonNull(fields, "version", "ok") ||
		json.Unmarshal(data, &r) != nil || r.Version != Version {
		return Reply{}, ErrProtocol
	}
	if r.OK {
		if r.Error != nil || isNull(r.Result) || validateResult(op, r.Result) != nil {
			return Reply{}, ErrProtocol
		}
	} else {
		fields, err := object(fields["error"], "code", "message")
		if err != nil || !nonNull(fields, "code", "message") || r.Error == nil || !isNull(r.Result) ||
			messages[r.Error.Code] != r.Error.Message || r.Error.Message == "" {
			return Reply{}, ErrProtocol
		}
	}
	return r, nil
}

func readFrame(r io.Reader, size int) ([]byte, error) {
	reader := bufio.NewReaderSize(r, min(size, 4096))
	var frame []byte
	for {
		data, err := reader.ReadSlice('\n')
		if len(frame)+len(data) > size {
			return nil, ErrProtocol
		}
		frame = append(frame, data...)
		if err == nil {
			return frame, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, ErrProtocol
		}
	}
}

func writeFrame(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func object(data []byte, names ...string) (map[string]json.RawMessage, error) {
	if !validJSONText(data) {
		return nil, ErrProtocol
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if token, err := dec.Token(); err != nil || token != json.Delim('{') {
		return nil, ErrProtocol
	}
	fields := make(map[string]json.RawMessage, len(names))
	for dec.More() {
		token, err := dec.Token()
		name, ok := token.(string)
		if err != nil || !ok || !slices.Contains(names, name) || fields[name] != nil {
			return nil, ErrProtocol
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, ErrProtocol
		}
		fields[name] = value
	}
	if token, err := dec.Token(); err != nil || token != json.Delim('}') || len(fields) != len(names) {
		return nil, ErrProtocol
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, ErrProtocol
	}
	return fields, nil
}

func nonNull(fields map[string]json.RawMessage, names ...string) bool {
	for _, name := range names {
		if isNull(fields[name]) {
			return false
		}
	}
	return true
}

func isNull(data []byte) bool {
	return len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

func validID(id string) bool {
	_, err := hex.DecodeString(id)
	return err == nil && len(id) == 32 && strings.ToLower(id) == id
}

func validPath(path string) bool {
	return len(path) <= queue.MaxPathSize && filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsRune(path, 0)
}

var messages = map[string]string{
	"invalid_request": "invalid daemon protocol message",
	"state_mismatch":  "daemon state directory does not match",
	"not_found":       "queue job was not found",
	"conflict":        "queue job conflicts with the requested operation",
	"busy":            "queue operation must wait for active work to finish",
	"capacity":        "queue retention or admission capacity is exhausted",
	"stopping":        "daemon is stopping; mutations are disabled",
	"unavailable":     "daemon is unavailable",
	"outcome_unknown": "queue mutation outcome is unknown; inspect queue state",
	"persistence":     "queue persistence failed; restart requires inspection",
	"internal":        "daemon operation failed",
}

func failure(err error) *Failure {
	code := "internal"
	switch {
	case errors.Is(err, queue.ErrOutcomeUnknown):
		code = "outcome_unknown"
	case errors.Is(err, queue.ErrPersistence):
		code = "persistence"
	case errors.Is(err, ErrProtocol), errors.Is(err, queue.ErrInvalid):
		code = "invalid_request"
	case errors.Is(err, ErrState):
		code = "state_mismatch"
	case errors.Is(err, queue.ErrNotFound):
		code = "not_found"
	case errors.Is(err, queue.ErrConflict):
		code = "conflict"
	case errors.Is(err, queue.ErrBusy):
		code = "busy"
	case errors.Is(err, queue.ErrCapacity), errors.Is(err, queue.ErrOrder):
		code = "capacity"
	case errors.Is(err, queue.ErrStopping):
		code = "stopping"
	case errors.Is(err, queue.ErrStopped), errors.Is(err, queue.ErrMailbox), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code = "unavailable"
	}
	return &Failure{Code: code, Message: messages[code]}
}
