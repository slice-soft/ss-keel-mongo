package mongo

import "time"

// EntityBase provides common document fields for MongoDB entities.
// Embed this in any entity to get ID, CreatedAt and UpdatedAt with BSON tags pre-configured.
//
// Call OnCreate before inserting a document and OnUpdate before updating one.
// The generated repository does this automatically when using the official CLI templates.
type EntityBase struct {
	ID        string `json:"id"         bson:"_id,omitempty"`
	CreatedAt int64  `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at" bson:"updated_at,omitempty"`
}

// OnCreate sets both CreatedAt and UpdatedAt to the current Unix millisecond timestamp.
// Call this before inserting a new document.
func (e *EntityBase) OnCreate() {
	now := time.Now().UnixMilli()
	e.CreatedAt = now
	e.UpdatedAt = now
}

// OnUpdate sets UpdatedAt to the current Unix millisecond timestamp.
// CreatedAt is left unchanged. Call this before updating an existing document.
func (e *EntityBase) OnUpdate() {
	e.UpdatedAt = time.Now().UnixMilli()
}
