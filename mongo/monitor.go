package mongo

import (
	"context"
	"sync"
	"time"

	"github.com/slice-soft/ss-keel-core/contracts"
	"go.mongodb.org/mongo-driver/event"
)

type mongoMonitor struct {
	starts sync.Map // int64 requestID -> time.Time
	emit   func(contracts.PanelEvent)
}

func newMongoMonitor(emit func(contracts.PanelEvent)) *mongoMonitor {
	return &mongoMonitor{emit: emit}
}

func (m *mongoMonitor) asCommandMonitor() *event.CommandMonitor {
	return &event.CommandMonitor{
		Started:   m.started,
		Succeeded: m.succeeded,
		Failed:    m.failed,
	}
}

func (m *mongoMonitor) started(_ context.Context, e *event.CommandStartedEvent) {
	m.starts.Store(e.RequestID, time.Now())
}

func (m *mongoMonitor) succeeded(_ context.Context, e *event.CommandSucceededEvent) {
	startVal, ok := m.starts.LoadAndDelete(e.RequestID)
	var durationMs int64
	if ok {
		durationMs = time.Since(startVal.(time.Time)).Milliseconds()
	}
	m.emit(contracts.PanelEvent{
		Timestamp: time.Now(),
		AddonID:   "mongo",
		Label:     e.CommandName,
		Level:     "info",
		Detail: map[string]any{
			"command":     e.CommandName,
			"duration_ms": durationMs,
		},
	})
}

func (m *mongoMonitor) failed(_ context.Context, e *event.CommandFailedEvent) {
	m.starts.LoadAndDelete(e.RequestID)
	m.emit(contracts.PanelEvent{
		Timestamp: time.Now(),
		AddonID:   "mongo",
		Label:     e.CommandName,
		Level:     "error",
		Detail: map[string]any{
			"command": e.CommandName,
			"failure": e.Failure,
		},
	})
}
