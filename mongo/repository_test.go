package mongo

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/slice-soft/ss-keel-core/core/httpx"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type repoUser struct {
	ID      string                 `bson:"_id,omitempty"`
	Name    string                 `bson:"name"`
	Profile map[string]interface{} `bson:"profile,omitempty"`
}

type fakeSingleResult struct {
	document interface{}
	err      error
}

func (r *fakeSingleResult) Decode(v interface{}) error {
	if r.err != nil {
		return r.err
	}

	if r.document == nil {
		return nil
	}

	payload, err := bson.Marshal(r.document)
	if err != nil {
		return err
	}

	return bson.Unmarshal(payload, v)
}

type fakeCursor struct {
	documents interface{}
	allErr    error
	closed    bool
}

func (c *fakeCursor) All(context.Context, interface{}) error {
	return c.allErr
}

func (c *fakeCursor) Close(context.Context) error {
	c.closed = true
	return nil
}

type fakeCollection struct {
	rawNil bool

	findOneResult   singleResult
	findResult      cursor
	findErr         error
	countResult     int64
	countErr        error
	insertErr       error
	updateErr       error
	deleteErr       error
	lastFindOneFilt interface{}
	lastFindFilt    interface{}
	lastFindOpts    []*options.FindOptions
	lastInsertDoc   interface{}
	lastUpdateFilt  interface{}
	lastUpdateDoc   interface{}
	lastDeleteFilt  interface{}
}

func (c *fakeCollection) FindOne(_ context.Context, filter interface{}, _ ...*options.FindOneOptions) singleResult {
	c.lastFindOneFilt = filter
	if c.findOneResult == nil {
		return &fakeSingleResult{err: mongodriver.ErrNoDocuments}
	}
	return c.findOneResult
}

func (c *fakeCollection) Find(_ context.Context, filter interface{}, opts ...*options.FindOptions) (cursor, error) {
	c.lastFindFilt = filter
	c.lastFindOpts = opts
	if c.findErr != nil {
		return nil, c.findErr
	}
	if c.findResult == nil {
		return &fakeCursor{}, nil
	}
	return c.findResult, nil
}

func (c *fakeCollection) InsertOne(_ context.Context, document interface{}, _ ...*options.InsertOneOptions) (*mongodriver.InsertOneResult, error) {
	c.lastInsertDoc = document
	if c.insertErr != nil {
		return nil, c.insertErr
	}
	return &mongodriver.InsertOneResult{}, nil
}

func (c *fakeCollection) UpdateOne(_ context.Context, filter interface{}, update interface{}, _ ...*options.UpdateOptions) (*mongodriver.UpdateResult, error) {
	c.lastUpdateFilt = filter
	c.lastUpdateDoc = update
	if c.updateErr != nil {
		return nil, c.updateErr
	}
	return &mongodriver.UpdateResult{}, nil
}

func (c *fakeCollection) DeleteOne(_ context.Context, filter interface{}, _ ...*options.DeleteOptions) (*mongodriver.DeleteResult, error) {
	c.lastDeleteFilt = filter
	if c.deleteErr != nil {
		return nil, c.deleteErr
	}
	return &mongodriver.DeleteResult{}, nil
}

func (c *fakeCollection) CountDocuments(_ context.Context, _ interface{}, _ ...*options.CountOptions) (int64, error) {
	if c.countErr != nil {
		return 0, c.countErr
	}
	return c.countResult, nil
}

func (c *fakeCollection) Raw() *mongodriver.Collection {
	if c.rawNil {
		return nil
	}
	return &mongodriver.Collection{}
}

func TestNewPage(t *testing.T) {
	page := httpx.NewPage([]int{1, 2, 3}, 8, 2, 3)

	if page.TotalPages != 3 {
		t.Fatalf("expected total pages 3, got %d", page.TotalPages)
	}
}

func TestMongoRepository_FindByIDReturnsNilWhenNotFound(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{
		findOneResult: &fakeSingleResult{err: mongodriver.ErrNoDocuments},
	})

	entity, err := repo.FindByID(context.Background(), "missing")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if entity != nil {
		t.Fatalf("expected nil entity, got %+v", entity)
	}
}

func TestMongoRepository_FindByIDDecodesDocument(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{
		findOneResult: &fakeSingleResult{document: bson.M{"_id": "u1", "name": "Ada"}},
	})

	entity, err := repo.FindByID(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if entity == nil || entity.Name != "Ada" {
		t.Fatalf("unexpected entity: %+v", entity)
	}
}

func TestMongoRepository_FindByID_DecodeError(t *testing.T) {
	wantErr := errors.New("decode error")
	repo := newRepository[repoUser, string](&fakeCollection{
		findOneResult: &fakeSingleResult{err: wantErr},
	})
	_, err := repo.FindByID(context.Background(), "u1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestMongoRepository_FindAllNormalizesPagination(t *testing.T) {
	cursor := &fakeCursor{}
	fake := &fakeCollection{
		countResult: 3,
		findResult:  cursor,
	}
	repo := newRepository[repoUser, string](fake)

	page, err := repo.FindAll(context.Background(), httpx.PageQuery{Page: 0, Limit: 200})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if page.Page != 1 {
		t.Fatalf("expected page 1 after normalization, got %d", page.Page)
	}
	if page.Limit != 100 {
		t.Fatalf("expected limit capped at 100, got %d", page.Limit)
	}

	if len(fake.lastFindOpts) != 1 {
		t.Fatalf("expected one find options entry, got %d", len(fake.lastFindOpts))
	}

	opt := fake.lastFindOpts[0]
	if opt.Skip == nil || *opt.Skip != 0 {
		t.Fatalf("unexpected skip value: %#v", opt.Skip)
	}
	if opt.Limit == nil || *opt.Limit != 100 {
		t.Fatalf("unexpected limit value: %#v", opt.Limit)
	}
	if opt.Sort == nil {
		t.Fatal("expected default sort to be set")
	}
	if !cursor.closed {
		t.Fatal("expected cursor to be closed")
	}
}

func TestMongoRepository_FindOneByFilter(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{
		findOneResult: &fakeSingleResult{document: bson.M{"_id": "u2", "name": "Grace"}},
	})

	entity, err := repo.FindOneByFilter(context.Background(), bson.M{"name": "Grace"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if entity == nil || entity.Name != "Grace" {
		t.Fatalf("unexpected entity: %+v", entity)
	}
}

func TestMongoRepository_FindManyUsesProvidedFilter(t *testing.T) {
	fake := &fakeCollection{findResult: &fakeCursor{}}
	repo := newRepository[repoUser, string](fake)

	filter := bson.M{"profile.active": true}
	_, err := repo.FindMany(context.Background(), filter, options.Find().SetLimit(5))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !reflect.DeepEqual(fake.lastFindFilt, filter) {
		t.Fatalf("unexpected find filter: got %#v want %#v", fake.lastFindFilt, filter)
	}
}

func TestMongoRepository_CreateValidatesEntity(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{})

	err := repo.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected entity validation error")
	}
}

func TestMongoRepository_UpdateStripsIDFieldFromSetDocument(t *testing.T) {
	fake := &fakeCollection{}
	repo := newRepository[repoUser, string](fake)
	entity := &repoUser{ID: "u1", Name: "Ada", Profile: map[string]interface{}{"active": true}}

	err := repo.Update(context.Background(), "u1", entity)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	filter, ok := fake.lastUpdateFilt.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M update filter, got %T", fake.lastUpdateFilt)
	}
	if filter["_id"] != "u1" {
		t.Fatalf("expected _id filter to be u1, got %#v", filter["_id"])
	}

	updateDoc, ok := fake.lastUpdateDoc.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M update document, got %T", fake.lastUpdateDoc)
	}
	setDoc, ok := updateDoc["$set"].(bson.M)
	if !ok {
		t.Fatalf("expected bson.M $set document, got %T", updateDoc["$set"])
	}
	if _, hasID := setDoc["_id"]; hasID {
		t.Fatalf("expected $set to exclude _id, got %#v", setDoc)
	}
	if setDoc["name"] != "Ada" {
		t.Fatalf("expected updated name to be Ada, got %#v", setDoc["name"])
	}
}

func TestMongoRepository_PatchStripsIDFieldFromSetDocument(t *testing.T) {
	fake := &fakeCollection{}
	repo := newRepository[repoUser, string](fake)
	patch := &repoUser{ID: "u1", Name: "Grace", Profile: map[string]interface{}{"active": true}}

	err := repo.Patch(context.Background(), "u1", patch)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	filter, ok := fake.lastUpdateFilt.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M update filter, got %T", fake.lastUpdateFilt)
	}
	if filter["_id"] != "u1" {
		t.Fatalf("expected _id filter to be u1, got %#v", filter["_id"])
	}

	updateDoc, ok := fake.lastUpdateDoc.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M update document, got %T", fake.lastUpdateDoc)
	}
	setDoc, ok := updateDoc["$set"].(bson.M)
	if !ok {
		t.Fatalf("expected bson.M $set document, got %T", updateDoc["$set"])
	}
	if _, hasID := setDoc["_id"]; hasID {
		t.Fatalf("expected $set to exclude _id, got %#v", setDoc)
	}
	if setDoc["name"] != "Grace" {
		t.Fatalf("expected patched name to be Grace, got %#v", setDoc["name"])
	}
}

func TestMongoRepository_PatchNilPatch(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{})
	err := repo.Patch(context.Background(), "u1", nil)
	if err == nil {
		t.Fatal("expected patch validation error")
	}
}

func TestMongoRepository_DeleteUsesIDFilter(t *testing.T) {
	fake := &fakeCollection{}
	repo := newRepository[repoUser, string](fake)

	err := repo.Delete(context.Background(), "u3")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	filter, ok := fake.lastDeleteFilt.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M delete filter, got %T", fake.lastDeleteFilt)
	}
	if filter["_id"] != "u3" {
		t.Fatalf("expected _id filter to be u3, got %#v", filter["_id"])
	}
}

func TestMongoRepository_CollectionNilReturnsError(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{rawNil: true})

	err := repo.Delete(context.Background(), "u3")
	if err == nil {
		t.Fatal("expected collection nil error")
	}
}

func TestMongoRepository_WithCustomIDFieldAndConverter(t *testing.T) {
	fake := &fakeCollection{}
	repo := newRepository[repoUser, string](
		fake,
		WithIDField[repoUser, string]("slug"),
		WithIDConverter[repoUser, string](func(id string) (interface{}, error) {
			return "prefix-" + id, nil
		}),
	)

	err := repo.Delete(context.Background(), "user")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	filter, ok := fake.lastDeleteFilt.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M filter, got %T", fake.lastDeleteFilt)
	}
	if filter["slug"] != "prefix-user" {
		t.Fatalf("unexpected custom filter value: %#v", filter)
	}
}

func TestMongoRepository_WithFailingIDConverterReturnsError(t *testing.T) {
	fake := &fakeCollection{}
	repo := newRepository[repoUser, string](
		fake,
		WithIDConverter[repoUser, string](func(id string) (interface{}, error) {
			return nil, errors.New("convert failure")
		}),
	)

	err := repo.Delete(context.Background(), "u4")
	if err == nil {
		t.Fatal("expected id converter error")
	}
}

// --- ensureReady error propagation for every public method ---

// TestMongoRepository_AllMethods_EnsureReadyError verifies that every method
// returns an error when the underlying collection is nil (rawNil: true),
// covering the "return ..., err" statement inside each function's ensureReady check.
func TestMongoRepository_AllMethods_EnsureReadyError(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{rawNil: true})
	ctx := context.Background()

	if _, err := repo.FindByID(ctx, "x"); err == nil {
		t.Fatal("FindByID: expected ensureReady error")
	}
	if _, err := repo.FindAll(ctx, httpx.PageQuery{}); err == nil {
		t.Fatal("FindAll: expected ensureReady error")
	}
	if _, err := repo.FindOneByFilter(ctx, bson.M{}); err == nil {
		t.Fatal("FindOneByFilter: expected ensureReady error")
	}
	if _, err := repo.FindMany(ctx, bson.M{}); err == nil {
		t.Fatal("FindMany: expected ensureReady error")
	}
	if err := repo.Create(ctx, &repoUser{}); err == nil {
		t.Fatal("Create: expected ensureReady error")
	}
	if err := repo.Update(ctx, "x", &repoUser{Name: "Ada"}); err == nil {
		t.Fatal("Update: expected ensureReady error")
	}
	if err := repo.Patch(ctx, "x", &repoUser{Name: "Ada"}); err == nil {
		t.Fatal("Patch: expected ensureReady error")
	}
}

// --- NewRepository / NewRepositoryFromCollection ---

func TestNewRepository_WithNilClient(t *testing.T) {
	repo := NewRepository[repoUser, string](nil, "users")
	// collection wraps a nil *mongodriver.Collection → ensureReady returns error
	err := repo.Delete(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for nil client repository")
	}
}

func TestNewRepositoryFromCollection_WithNilCollection(t *testing.T) {
	// Also exercises mongoCollection.Raw() via ensureReady.
	repo := NewRepositoryFromCollection[repoUser, string](nil)
	err := repo.Delete(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error for nil collection repository")
	}
}

// --- MongoRepository.Collection ---

func TestMongoRepository_CollectionMethod(t *testing.T) {
	// nil receiver
	var r *MongoRepository[repoUser, string]
	if r.Collection() != nil {
		t.Fatal("expected nil for nil receiver")
	}

	// zero-value repo (collection field is nil interface)
	r2 := &MongoRepository[repoUser, string]{}
	if r2.Collection() != nil {
		t.Fatal("expected nil when collection is nil")
	}

	// repo with a fake collection → returns non-nil raw handle
	r3 := newRepository[repoUser, string](&fakeCollection{})
	if r3.Collection() == nil {
		t.Fatal("expected non-nil collection from fake")
	}
}

// --- FindOneByFilter ---

func TestMongoRepository_FindOneByFilter_NotFound(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{
		findOneResult: &fakeSingleResult{err: mongodriver.ErrNoDocuments},
	})

	entity, err := repo.FindOneByFilter(context.Background(), bson.M{"name": "missing"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if entity != nil {
		t.Fatalf("expected nil entity, got %+v", entity)
	}
}

func TestMongoRepository_FindOneByFilter_DecodeError(t *testing.T) {
	wantErr := errors.New("decode error")
	repo := newRepository[repoUser, string](&fakeCollection{
		findOneResult: &fakeSingleResult{err: wantErr},
	})

	_, err := repo.FindOneByFilter(context.Background(), bson.M{"name": "x"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestMongoRepository_FindOneByFilter_NilFilter(t *testing.T) {
	// nil filter is normalised to bson.D{}, covering normalizeFilter's nil branch.
	repo := newRepository[repoUser, string](&fakeCollection{
		findOneResult: &fakeSingleResult{err: mongodriver.ErrNoDocuments},
	})
	_, err := repo.FindOneByFilter(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// --- FindMany ---

func TestMongoRepository_FindMany_FindError(t *testing.T) {
	wantErr := errors.New("find error")
	repo := newRepository[repoUser, string](&fakeCollection{findErr: wantErr})

	_, err := repo.FindMany(context.Background(), bson.M{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestMongoRepository_FindMany_CursorAllError(t *testing.T) {
	wantErr := errors.New("cursor all error")
	repo := newRepository[repoUser, string](&fakeCollection{
		findResult: &fakeCursor{allErr: wantErr},
	})

	_, err := repo.FindMany(context.Background(), bson.M{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected cursor all error, got %v", err)
	}
}

// --- Create ---

func TestMongoRepository_Create_Success(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{})
	err := repo.Create(context.Background(), &repoUser{Name: "Ada"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestMongoRepository_Create_InsertError(t *testing.T) {
	wantErr := errors.New("insert failed")
	repo := newRepository[repoUser, string](&fakeCollection{insertErr: wantErr})

	err := repo.Create(context.Background(), &repoUser{Name: "Ada"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected insert error, got %v", err)
	}
}

// --- Update ---

func TestMongoRepository_Update_NilEntity(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{})
	err := repo.Update(context.Background(), "u1", nil)
	if err == nil {
		t.Fatal("expected entity required error")
	}
}

func TestMongoRepository_Update_PropagatesUpdateError(t *testing.T) {
	wantErr := errors.New("update failed")
	repo := newRepository[repoUser, string](&fakeCollection{updateErr: wantErr})
	entity := &repoUser{Name: "Ada"}

	err := repo.Update(context.Background(), "u1", entity)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected update error, got %v", err)
	}
}

func TestMongoRepository_Update_IDConverterError(t *testing.T) {
	repo := newRepository[repoUser, string](
		&fakeCollection{},
		WithIDConverter[repoUser, string](func(id string) (interface{}, error) {
			return nil, errors.New("bad id")
		}),
	)

	err := repo.Update(context.Background(), "u1", &repoUser{Name: "Ada"})
	if err == nil {
		t.Fatal("expected id converter error")
	}
}

// onlyIDEntity is used to trigger the "empty update document" error path:
// after marshalling and removing the single _id field the doc is empty.
type onlyIDEntity struct {
	ID string `bson:"_id"`
}

func TestMongoRepository_Update_EmptyDocError(t *testing.T) {
	repo := newRepository[onlyIDEntity, string](&fakeCollection{})
	err := repo.Update(context.Background(), "test", &onlyIDEntity{ID: "test"})
	if err == nil {
		t.Fatal("expected error for empty update document")
	}
}

// --- documentForUpdate ---

// marshalErrorEntity implements bson.Marshaler and always returns an error,
// allowing us to cover the bson.Marshal failure path in documentForUpdate.
type marshalErrorEntity struct{}

func (marshalErrorEntity) MarshalBSON() ([]byte, error) {
	return nil, errors.New("forced marshal error")
}

func TestDocumentForUpdate_MarshalError(t *testing.T) {
	_, err := documentForUpdate(marshalErrorEntity{}, "_id")
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestDocumentForUpdate_EmptyDocReturnsError(t *testing.T) {
	entity := &onlyIDEntity{ID: "test"}
	_, err := documentForUpdate(entity, "_id")
	if err == nil {
		t.Fatal("expected error when update document is empty after stripping id field")
	}
}

// --- normalizeFilter ---

func TestNormalizeFilter_NilReturnsEmptyBSONDoc(t *testing.T) {
	got := normalizeFilter(nil)
	if _, ok := got.(bson.D); !ok {
		t.Fatalf("expected bson.D for nil filter, got %T", got)
	}
}

// --- normalizePageQuery ---

func TestNormalizePageQuery_ZeroLimitDefaultsTwenty(t *testing.T) {
	q := normalizePageQuery(httpx.PageQuery{Page: 1, Limit: 0})
	if q.Limit != 20 {
		t.Fatalf("expected default limit 20, got %d", q.Limit)
	}
}

// --- FindAll error paths ---

func TestMongoRepository_FindAll_CountError(t *testing.T) {
	wantErr := errors.New("count failed")
	repo := newRepository[repoUser, string](&fakeCollection{countErr: wantErr})

	_, err := repo.FindAll(context.Background(), httpx.PageQuery{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected count error, got %v", err)
	}
}

func TestMongoRepository_FindAll_FindError(t *testing.T) {
	wantErr := errors.New("find failed")
	repo := newRepository[repoUser, string](&fakeCollection{countResult: 3, findErr: wantErr})

	_, err := repo.FindAll(context.Background(), httpx.PageQuery{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestMongoRepository_FindAll_CursorAllError(t *testing.T) {
	wantErr := errors.New("cursor all error")
	repo := newRepository[repoUser, string](&fakeCollection{
		countResult: 3,
		findResult:  &fakeCursor{allErr: wantErr},
	})

	_, err := repo.FindAll(context.Background(), httpx.PageQuery{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected cursor all error, got %v", err)
	}
}

func TestMongoRepository_FindAll_WithNoDefaultSort(t *testing.T) {
	// WithDefaultSort(nil) disables sorting; the SetSort branch must not be entered.
	fake := &fakeCollection{findResult: &fakeCursor{}}
	repo := newRepository[repoUser, string](fake, WithDefaultSort[repoUser, string](nil))

	_, err := repo.FindAll(context.Background(), httpx.PageQuery{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(fake.lastFindOpts) > 0 && fake.lastFindOpts[0].Sort != nil {
		t.Fatal("expected no sort when WithDefaultSort(nil) is used")
	}
}

// --- WithDefaultSort ---

func TestWithDefaultSort_SetsCustomSort(t *testing.T) {
	sort := bson.D{{Key: "name", Value: -1}}
	repo := newRepository[repoUser, string](&fakeCollection{}, WithDefaultSort[repoUser, string](sort))
	if repo.defaultSort == nil {
		t.Fatal("expected default sort to be set")
	}
}

// --- WithIDField ---

func TestWithIDField_EmptyStringKeepsDefault(t *testing.T) {
	repo := newRepository[repoUser, string](&fakeCollection{}, WithIDField[repoUser, string]("   "))
	if repo.idField != "_id" {
		t.Fatalf("expected default id field _id, got %q", repo.idField)
	}
}

// --- WithIDConverter ---

func TestWithIDConverter_NilConverterIgnored(t *testing.T) {
	fake := &fakeCollection{}
	repo := newRepository[repoUser, string](fake, WithIDConverter[repoUser, string](nil))

	// Default converter is kept (passes id as-is).
	err := repo.Delete(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil error with default converter, got %v", err)
	}
	filter, ok := fake.lastDeleteFilt.(bson.M)
	if !ok || filter["_id"] != "u1" {
		t.Fatalf("expected default _id filter, got %#v", fake.lastDeleteFilt)
	}
}

// --- newRepository with nil option ---

func TestNewRepository_NilOptionIsIgnored(t *testing.T) {
	var nilOpt RepositoryOption[repoUser, string]
	repo := newRepository[repoUser, string](&fakeCollection{}, nilOpt)
	if repo == nil {
		t.Fatal("expected non-nil repository even with nil option")
	}
}
