package queue

import (
	"context"
	"errors"
	"math"
	"os"

	"yoinkdl/internal/transfer"
)

var ErrConflict = errors.New("queue job conflicts with the requested operation")
var ErrBusy = errors.New("queue operation must wait for active work to finish")
var ErrOrder = errors.New("queue admission order is exhausted")

const interruptedMessage = "previous attempt ended without a recorded result"
const failedMessage = "download failed"
const publishedMessage = "download failed after publishing output"
const unpublishedMessage = "download ended without publishing verified output"
const canceledMessage = "download was canceled"
const timeoutMessage = "download timed out"
const accessMessage = "download could not access local files"

func recoverJobs(s *Snapshot) bool {
	changed := false
	for i := range s.Jobs {
		j := &s.Jobs[i]
		switch j.State {
		case Running:
			j.State = Interrupted
			message := interruptedMessage
			j.Error = &message
		case Pausing:
			j.State = Paused
			j.Error = nil
		case Canceling:
			j.State = Canceled
			j.Request.URL = nil
			j.Error = nil
		default:
			continue
		}

		j.Published = nil
		changed = true
	}
	return changed
}

func pauseJob(j *Job) (bool, error) {
	switch j.State {
	case Queued:
		j.State = Paused
		return true, nil
	case Running:
		j.State = Pausing
		return true, nil
	case Pausing, Paused:
		return false, nil
	default:
		return false, ErrConflict
	}
}

func resumeJob(j *Job, next *int64) (bool, error) {
	switch j.State {
	case Queued, Running:
		return false, nil
	case Pausing, Canceling:
		return false, ErrBusy
	case Paused, Interrupted, Failed:
		if *next == math.MaxInt64 {
			return false, ErrOrder
		}

		j.Order = *next
		*next += 1
		j.State = Queued
		j.Error = nil
		return true, nil
	default:
		return false, ErrConflict
	}
}

func cancelJob(j *Job) (bool, error) {
	switch j.State {
	case Completed:
		return false, ErrConflict
	case Canceled, Canceling:
		return false, nil
	case Running, Pausing:
		j.State = Canceling
	default:
		j.State = Canceled
		j.Request.URL = nil
		j.Error = nil
	}
	return true, nil
}

func finishJob(j Job, result transfer.Result, err error) Job {
	published := result.Published
	j.Published = &published
	j.Error = nil
	if err == nil && published {
		j.State = Completed
		j.Request.URL = nil
		return j
	}

	if !published && errors.Is(err, context.Canceled) {
		switch j.State {
		case Pausing:
			j.State = Paused
			return j
		case Canceling:
			j.State = Canceled
			j.Request.URL = nil
			return j
		}
	}

	message := failedMessage
	switch {
	case published:
		message = publishedMessage
	case err == nil:
		message = unpublishedMessage
	case errors.Is(err, context.Canceled):
		message = canceledMessage
	case errors.Is(err, context.DeadlineExceeded):
		message = timeoutMessage
	case errors.Is(err, os.ErrPermission):
		message = accessMessage
	}
	j.State = Failed
	j.Error = &message
	return j
}

func publicError(j Job) *string {
	if j.Error == nil {
		return nil
	}

	message := failedMessage
	if j.State == Interrupted {
		message = interruptedMessage
	} else {
		switch *j.Error {
		case failedMessage, publishedMessage, unpublishedMessage, canceledMessage, timeoutMessage, accessMessage:
			message = *j.Error
		}
	}
	return &message
}
