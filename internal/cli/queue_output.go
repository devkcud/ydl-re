package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"yoinkdl/internal/daemon"
	"yoinkdl/internal/queue"
)

type queueRecord struct {
	Version  int             `json:"schema_version"`
	Command  string          `json:"command"`
	ID       *string         `json:"job_id"`
	ExitCode int             `json:"exit_code"`
	Result   json.RawMessage `json:"result"`
	Error    *daemon.Failure `json:"error"`
}

func queueText(cmd queueCall, data []byte) (string, error) {
	switch cmd.op {
	case "add":
		return cmd.id + "\n", nil
	case "list":
		var view daemon.List
		if err := json.Unmarshal(data, &view); err != nil {
			return "", daemon.ErrProtocol
		}
		var text strings.Builder
		text.WriteString(daemonText(view.Daemon))
		if len(view.Jobs) == 0 {
			text.WriteString("No jobs.\n")
		}
		for _, j := range view.Jobs {
			workers, published := "unknown", "unknown"
			if j.Workers != nil {
				workers = fmt.Sprint(*j.Workers)
			}
			if j.Published != nil {
				published = fmt.Sprint(*j.Published)
			}
			fmt.Fprintf(&text, "%s\t%s\t%s/%s bytes\tworkers=%s\tpublished=%s\t%s",
				j.ID, j.State, queueCount(j.Bytes), queueCount(j.Total), workers, published, safeText(j.Output))
			if j.Error != nil {
				fmt.Fprintf(&text, "\t%s", safeText(*j.Error))
			}
			text.WriteByte('\n')
		}
		return text.String(), nil
	case "stop", "set_parallel":
		var status daemon.Status
		if json.Unmarshal(data, &status) != nil {
			return "", daemon.ErrProtocol
		}
		return daemonText(status), nil
	case "forget":
		var result daemon.Forgotten
		if json.Unmarshal(data, &result) != nil {
			return "", daemon.ErrProtocol
		}
		state := "absent"
		if result.Forgotten {
			state = "forgotten"
		}
		return cmd.id + " " + state + "\n", nil
	default:
		var j queue.JobView
		if json.Unmarshal(data, &j) != nil {
			return "", daemon.ErrProtocol
		}
		return cmd.id + " " + string(j.State) + "\n", nil
	}
}

func daemonText(s daemon.Status) string {
	limit := "unlimited"
	if s.LimitBPS != 0 {
		limit = fmt.Sprintf("%d B/s", s.LimitBPS)
	}
	return fmt.Sprintf("Daemon: %s; parallel: %d; active: %d; limit: %s\n", s.State, s.Parallel, s.Active, limit)
}

func queueCount(n *int64) string {
	if n == nil {
		return "unknown"
	}
	return fmt.Sprint(*n)
}

func queueFailure(err error) *daemon.Failure {
	var remote *daemon.Failure
	if errors.As(err, &remote) && remote.Code != "outcome_unknown" {
		copy := *remote
		return &copy
	}
	code, message := "failed", "daemon operation failed; inspect private state and runtime settings"
	switch {
	case errors.Is(err, queue.ErrOutcomeUnknown), remote != nil && remote.Code == "outcome_unknown":
		code, message = "outcome_unknown", "operation outcome is unknown; run yoinkdl list before retrying"
	case errors.Is(err, queue.ErrPersistence), errors.Is(err, queue.ErrUncertain):
		code, message = "persistence", "queue persistence failed; inspect state before restarting"
	case errors.Is(err, daemon.ErrChanged), errors.Is(err, queue.ErrChanged):
		code, message = "ownership", "daemon ownership or state changed; inspect before restarting"
	case errors.Is(err, daemon.ErrUnsupported):
		code, message = "unsupported", daemon.ErrUnsupported.Error()
	case errors.Is(err, daemon.ErrPaths):
		code, message = "paths", daemon.ErrPaths.Error()
	case errors.Is(err, daemon.ErrAccess), errors.Is(err, os.ErrPermission):
		code, message = "access", daemon.ErrAccess.Error()
	case errors.Is(err, daemon.ErrPeer):
		code, message = "peer", daemon.ErrPeer.Error()
	case errors.Is(err, daemon.ErrLocked), errors.Is(err, queue.ErrLocked):
		code, message = "locked", "daemon state or endpoint is already owned"
	case errors.Is(err, daemon.ErrEndpoint):
		code, message = "endpoint", daemon.ErrEndpoint.Error()
	case errors.Is(err, daemon.ErrProtocol):
		code, message = "protocol", "daemon request or reply is invalid"
	case errors.Is(err, context.Canceled):
		code, message = "interrupted", "operation interrupted"
	case errors.Is(err, context.DeadlineExceeded):
		code, message = "timeout", "operation timed out"
	case errors.Is(err, daemon.ErrConnection):
		code, message = "connection", "daemon connection failed; check startup and state/runtime settings"
	}
	return &daemon.Failure{Code: code, Message: message}
}

func queueExit(ctx context.Context, failure *daemon.Failure) int {
	if failure.Code != "persistence" && failure.Code != "ownership" &&
		(errors.Is(ctx.Err(), context.Canceled) || failure.Code == "interrupted") {
		return 130
	}
	return 1
}

func queueErrorText(cmd queueCall, failure *daemon.Failure) string {
	text := cmd.name + " failed: " + failure.Message
	if cmd.id != "" {
		text += "\nJob ID: " + cmd.id
	}
	return text
}
