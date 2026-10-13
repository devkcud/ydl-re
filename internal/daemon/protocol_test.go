package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

const testID = "00112233445566778899aabbccddeeff"

func TestProtocolCommands(t *testing.T) {
	j, err := queue.NewJob(testID, 1, transfer.Request{URL: "https://example.org/file?secret=private", Output: "/tmp/file"})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"ping", "list", "stop", "add", "pause", "resume", "cancel", "forget", "set_parallel"} {
		t.Run(op, func(t *testing.T) {
			args := map[string]any{}
			switch op {
			case "add":
				args["request"] = j.Request
				args["id"] = testID
			case "pause", "resume", "cancel", "forget":
				args["id"] = testID
			case "set_parallel":
				args["parallel"] = queue.MaxParallel
			}
			data := requestFrame(t, "/state", op, args)
			cmd, err := decodeCommand(data, "/state")
			if err != nil || cmd.op != op || op == "add" && (cmd.req.URL != *j.Request.URL || cmd.id != testID || cmd.req.Overwrite) ||
				op == "set_parallel" && cmd.parallel != queue.MaxParallel {
				t.Fatalf("command = %+v, %v", cmd, err)
			}
		})
	}
}

func TestProtocolRejectsMalformedPrivateRequests(t *testing.T) {
	const source = `{"version":1,"state_dir":"/state","op":"add","args":{"id":"` + testID + `","request":{"url":"https://example.org/file?secret=private","output":"/tmp/file","connections":4,"retries":3,"timeout_ns":30000000000,"limit_bps":0,"resume":false}}}`
	for _, test := range []struct{ old, next string }{
		{`"version":1`, `"version":2`},
		{`"version":1`, `"version":null`},
		{`"version":1`, `"version":1,"version":1`},
		{`"version":1`, `"version":1.0`},
		{`"state_dir":"/state"`, `"state_dir":null`},
		{`"state_dir":"/state"`, `"state_dir":"relative"`},
		{`"op":"add"`, `"op":"private"`},
		{`"op":"add"`, `"op":"ping"`},
		{`"op":"add"`, `"op":null`},
		{`"id":"` + testID + `"`, `"id":null`},
		{`"id":"` + testID + `"`, `"id":"BAD"`},
		{`"url":`, `"url":"private","\u0075rl":`},
		{`"output":"/tmp/file"`, `"output":"relative"`},
		{`"connections":4`, `"connections":0`},
		{`"retries":3`, `"retries":null`},
		{`"resume":false`, `"resume":false,"overwrite":true`},
		{`"resume":false`, `"resume":"false"`},
		{`"limit_bps":0`, `"limit_bps":-1`},
		{`"timeout_ns":30000000000`, `"timeout_ns":9223372036854775808`},
		{`secret=private`, `secret=\ud800`},
		{`secret=private`, "secret=\xff"},
		{`"url":"https://example.org/file?secret=private"`, `"url":null`},
		{`"url":"https://example.org/file?secret=private"`, `"url":"https://u:private@example.org/file"`},
	} {
		t.Run(test.next, func(t *testing.T) {
			data := strings.Replace(source, test.old, test.next, 1)
			if _, err := decodeCommand([]byte(data), "/state"); err == nil || strings.Contains(err.Error(), "private") {
				t.Fatalf("invalid/private request error = %v", err)
			}
		})
	}
	for _, data := range []string{"null", "[]", "{}", source + " {}", source + strings.Repeat(" ", MaxRequestSize)} {
		if _, err := decodeCommand([]byte(data), "/state"); !errors.Is(err, ErrProtocol) {
			t.Fatal("malformed, trailing, or oversized JSON accepted")
		}
	}
	if _, err := decodeCommand([]byte(source), "/other"); !errors.Is(err, ErrState) {
		t.Fatal("state mismatch accepted")
	}
	for _, parallel := range []any{nil, 0, 9, 1.5, "1"} {
		if _, err := decodeCommand(requestFrame(t, "/state", "set_parallel", map[string]any{"parallel": parallel}), "/state"); err == nil {
			t.Fatal("invalid parallel accepted")
		}
	}
	for _, args := range []any{nil, []any{}, map[string]any{"extra": "private"}} {
		if _, err := decodeCommand(requestFrame(t, "/state", "ping", args), "/state"); err == nil {
			t.Fatal("invalid empty args accepted")
		}
	}
}

func TestProtocolReplyShapesAndPrivacy(t *testing.T) {
	status := Status{Status: queue.Status{State: "running", Parallel: 1}, LimitBPS: 100}
	job := queue.JobView{ID: testID, Order: 1, State: queue.Paused, Output: "/tmp/file"}
	for op, result := range map[string]any{
		"ping": status, "stop": status, "set_parallel": status,
		"list": List{Daemon: status, Jobs: []queue.JobView{job}},
		"add":  job, "pause": job, "resume": job, "cancel": job,
		"forget": Forgotten{ID: testID, Forgotten: true},
	} {
		data := encodeReply(result, nil)
		if r, err := decodeReply(data, op); err != nil || !r.OK || r.Error != nil {
			t.Fatalf("%s reply = %+v, %v", op, r, err)
		}
	}
	for _, err := range []error{ErrProtocol, ErrState, queue.ErrOutcomeUnknown, queue.ErrPersistence, queue.ErrInvalid,
		queue.ErrNotFound, queue.ErrConflict, queue.ErrBusy, queue.ErrCapacity, queue.ErrOrder, queue.ErrStopping,
		queue.ErrStopped, queue.ErrMailbox, context.Canceled, context.DeadlineExceeded, errors.New("private")} {
		data := encodeReply(nil, fmt.Errorf("https://example.org/?secret=private: %w", err))
		if bytes.Contains(data, []byte("private")) || bytes.Contains(data, []byte("https://")) {
			t.Fatal("raw error reached a reply")
		}
		if r, err := decodeReply(data, "ping"); err != nil || r.OK || r.Error == nil {
			t.Fatalf("failure reply = %+v, %v", r, err)
		}
	}
	base := string(encodeReply(status, nil))
	for _, test := range []struct{ old, next string }{
		{`"ok":true`, `"ok":null`}, {`"ok":true`, `"ok":false`},
		{`"version":1`, `"version":2`}, {`"active":0`, `"active":9`},
		{`"active":0`, `"active":null`}, {`"state":"running"`, `"state":"unknown"`},
		{`"limit_bps":100`, `"limit_bps":-1`},
		{`"parallel":1`, `"parallel":1,"url":"private"`},
		{`"error":null`, `"error":{"code":"internal","message":"private"}`},
	} {
		if _, err := decodeReply([]byte(strings.Replace(base, test.old, test.next, 1)), "ping"); err == nil {
			t.Fatal("malformed reply accepted")
		}
	}
	data := encodeReply(strings.Repeat("x", MaxReplySize), nil)
	if len(data) > MaxReplySize {
		t.Fatal("reply size is unbounded")
	}
	if r, err := decodeReply(data, "ping"); err != nil || r.OK {
		t.Fatal("oversized reply was not a bounded failure")
	}
}

func TestProtocolFrameBoundaries(t *testing.T) {
	for _, size := range []int{MaxRequestSize, MaxReplySize} {
		frame := append(bytes.Repeat([]byte(" "), size-1), '\n')
		if data, err := readFrame(bytes.NewReader(frame), size); err != nil || len(data) != size {
			t.Fatal("exact frame boundary refused")
		}
		for _, data := range [][]byte{frame[:size-1], append([]byte(" "), frame...), nil} {
			if _, err := readFrame(bytes.NewReader(data), size); !errors.Is(err, ErrProtocol) {
				t.Fatal("missing newline or oversized frame accepted")
			}
		}
	}
	if data, err := readFrame(strings.NewReader("{}\n{}\n"), MaxRequestSize); err != nil || string(data) != "{}\n" {
		t.Fatal("reader did not isolate the first frame")
	}
}

func requestFrame(t *testing.T, state, op string, args any) []byte {
	t.Helper()
	data, err := json.Marshal(struct {
		Version  int    `json:"version"`
		StateDir string `json:"state_dir"`
		Op       string `json:"op"`
		Args     any    `json:"args"`
	}{Version, state, op, args})
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}
