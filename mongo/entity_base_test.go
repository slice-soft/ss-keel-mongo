package mongo

import (
	"testing"
	"time"
)

func TestEntityBaseOnCreate(t *testing.T) {
	before := time.Now().UnixMilli()

	var e EntityBase
	e.OnCreate()

	after := time.Now().UnixMilli()

	if e.CreatedAt < before || e.CreatedAt > after {
		t.Fatalf("CreatedAt %d not in expected range [%d, %d]", e.CreatedAt, before, after)
	}
	if e.UpdatedAt != e.CreatedAt {
		t.Fatalf("expected UpdatedAt == CreatedAt on create, got CreatedAt=%d UpdatedAt=%d", e.CreatedAt, e.UpdatedAt)
	}
}

func TestEntityBaseOnUpdate(t *testing.T) {
	var e EntityBase
	e.OnCreate()

	originalCreatedAt := e.CreatedAt

	time.Sleep(2 * time.Millisecond)

	before := time.Now().UnixMilli()
	e.OnUpdate()
	after := time.Now().UnixMilli()

	if e.CreatedAt != originalCreatedAt {
		t.Fatalf("OnUpdate must not change CreatedAt: got %d, want %d", e.CreatedAt, originalCreatedAt)
	}
	if e.UpdatedAt < before || e.UpdatedAt > after {
		t.Fatalf("UpdatedAt %d not in expected range [%d, %d]", e.UpdatedAt, before, after)
	}
	if e.UpdatedAt <= e.CreatedAt {
		t.Fatalf("expected UpdatedAt > CreatedAt after update, got CreatedAt=%d UpdatedAt=%d", e.CreatedAt, e.UpdatedAt)
	}
}
