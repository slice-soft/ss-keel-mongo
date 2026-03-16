package mongo

import (
	"testing"
	"time"
)

func TestEntityBaseOnCreate_GeneratesUUID(t *testing.T) {
	var e EntityBase
	e.OnCreate()

	if e.ID == "" {
		t.Fatal("expected ID to be generated, got empty string")
	}
}

func TestEntityBaseOnCreate_PreservesExistingID(t *testing.T) {
	const customID = "custom-id-123"
	e := EntityBase{ID: customID}
	e.OnCreate()

	if e.ID != customID {
		t.Fatalf("expected ID %q, got %q", customID, e.ID)
	}
}

func TestEntityBaseOnCreate_UniqueIDsPerRecord(t *testing.T) {
	var a, b EntityBase
	a.OnCreate()
	b.OnCreate()

	if a.ID == b.ID {
		t.Fatalf("expected unique IDs, both got %q", a.ID)
	}
}

func TestEntityBaseOnCreate_Timestamps(t *testing.T) {
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
