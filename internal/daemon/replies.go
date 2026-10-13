package daemon

import (
	"encoding/json"
	"math"

	"yoinkdl/internal/queue"
	"yoinkdl/internal/transfer"
)

func validateResult(op string, data []byte) error {
	switch op {
	case "ping", "stop", "set_parallel":
		return validateStatus(data)
	case "list":
		fields, err := object(data, "daemon", "jobs")
		if err != nil || validateStatus(fields["daemon"]) != nil {
			return ErrProtocol
		}
		var jobs []json.RawMessage
		if json.Unmarshal(fields["jobs"], &jobs) != nil || jobs == nil || len(jobs) > queue.MaxJobs {
			return ErrProtocol
		}
		ids := make(map[string]bool, len(jobs))
		var order int64
		for _, data := range jobs {
			if validateJob(data) != nil {
				return ErrProtocol
			}
			var job queue.JobView
			if json.Unmarshal(data, &job) != nil || ids[job.ID] || job.Order <= order {
				return ErrProtocol
			}
			ids[job.ID], order = true, job.Order
		}
		return nil
	case "add", "pause", "resume", "cancel":
		return validateJob(data)
	case "forget":
		fields, err := object(data, "id", "forgotten")
		var view Forgotten
		if err != nil || !nonNull(fields, "id", "forgotten") || json.Unmarshal(data, &view) != nil || !validID(view.ID) {
			return ErrProtocol
		}
		return nil
	default:
		return ErrProtocol
	}
}

func validateStatus(data []byte) error {
	keys := []string{"state", "parallel", "active", "limit_bps"}
	fields, err := object(data, keys...)
	var s Status
	if err != nil || !nonNull(fields, keys...) || json.Unmarshal(data, &s) != nil ||
		s.State != "running" && s.State != "stopping" || s.Parallel < 1 || s.Parallel > queue.MaxParallel ||
		s.Active < 0 || s.Active > queue.MaxParallel || s.LimitBPS < 0 {
		return ErrProtocol
	}
	return nil
}

func validateJob(data []byte) error {
	fields, err := object(data, "id", "order", "state", "output", "published", "error", "bytes", "total", "workers")
	var j queue.JobView
	if err != nil || !nonNull(fields, "id", "order", "state", "output") || json.Unmarshal(data, &j) != nil ||
		!validID(j.ID) || j.Order <= 0 || j.Order == math.MaxInt64 || !validPath(j.Output) ||
		j.Error != nil && (len(*j.Error) == 0 || len(*j.Error) > queue.MaxErrorSize) ||
		j.Bytes != nil && *j.Bytes < 0 || j.Total != nil && *j.Total < 0 ||
		j.Workers != nil && (*j.Workers < 0 || *j.Workers > transfer.MaxConnections) {
		return ErrProtocol
	}
	switch j.State {
	case queue.Queued, queue.Running, queue.Pausing, queue.Paused, queue.Canceling, queue.Canceled, queue.Interrupted, queue.Failed, queue.Completed:
		return nil
	default:
		return ErrProtocol
	}
}
