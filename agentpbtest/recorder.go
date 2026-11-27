package agentpbtest

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/allodium-co/contract/agentpb"
)

// ErrStreamEnded is returned when a stream ends while a caller is still
// waiting on it or sending on it. It wraps the error the stream ended with.
var ErrStreamEnded = errors.New("agentpbtest: stream ended")

func endedError(when string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w %s", ErrStreamEnded, when)
	}
	return fmt.Errorf("%w %s: %w", ErrStreamEnded, when, cause)
}

// recorder keeps the frames one side has received, in arrival order, and
// wakes waiters on every new frame and when the stream ends.
type recorder struct {
	mu      sync.Mutex
	frames  []*agentpb.AgentMessage
	changed chan struct{}
	ended   bool
	err     error
}

func newRecorder() *recorder {
	return &recorder{changed: make(chan struct{})}
}

func (r *recorder) add(m *agentpb.AgentMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = append(r.frames, m)
	r.notifyLocked()
}

// end marks the stream as finished with err. Only the first call counts.
func (r *recorder) end(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ended {
		return
	}
	r.ended = true
	r.err = err
	r.notifyLocked()
}

func (r *recorder) notifyLocked() {
	close(r.changed)
	r.changed = make(chan struct{})
}

func (r *recorder) snapshot() []*agentpb.AgentMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*agentpb.AgentMessage(nil), r.frames...)
}

func (r *recorder) endErr() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// waitFor returns the first recorded frame that match accepts, including
// frames recorded before the call. It does not consume frames: two calls with
// the same match return the same frame.
func (r *recorder) waitFor(ctx context.Context, match func(*agentpb.AgentMessage) bool) (*agentpb.AgentMessage, error) {
	return r.waitFrom(ctx, 0, match)
}

// waitForNext is waitFor over only the frames recorded after the call, for
// waiting on the effect of something the test just did.
func (r *recorder) waitForNext(ctx context.Context, match func(*agentpb.AgentMessage) bool) (*agentpb.AgentMessage, error) {
	r.mu.Lock()
	next := len(r.frames)
	r.mu.Unlock()
	return r.waitFrom(ctx, next, match)
}

// waitFrom runs match without the lock held, so match may call back into the
// fake.
func (r *recorder) waitFrom(ctx context.Context, next int, match func(*agentpb.AgentMessage) bool) (*agentpb.AgentMessage, error) {
	for {
		r.mu.Lock()
		frames := r.frames[next:len(r.frames):len(r.frames)]
		ended, err, changed := r.ended, r.err, r.changed
		r.mu.Unlock()

		for _, m := range frames {
			if match(m) {
				return m, nil
			}
		}
		next += len(frames)
		if ended {
			return nil, endedError("before a matching frame arrived", err)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for a matching frame: %w", ctx.Err())
		case <-changed:
		}
	}
}
