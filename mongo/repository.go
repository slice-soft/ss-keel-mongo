package mongo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/slice-soft/ss-keel-core/contracts"
	"github.com/slice-soft/ss-keel-core/core/httpx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IDConverter translates domain IDs into Mongo filters.
// Use this when API IDs differ from stored BSON IDs.
type IDConverter[ID any] func(id ID) (interface{}, error)

// ObjectIDHexConverter turns a hex string into primitive.ObjectID.
func ObjectIDHexConverter(id string) (interface{}, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid ObjectID hex %q: %w", id, err)
	}
	return oid, nil
}

type singleResult interface {
	Decode(v interface{}) error
}

type cursor interface {
	All(ctx context.Context, results interface{}) error
	Close(ctx context.Context) error
}

type collection interface {
	FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult
	Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (cursor, error)
	InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongodriver.InsertOneResult, error)
	UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongodriver.UpdateResult, error)
	DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongodriver.DeleteResult, error)
	CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error)
	Raw() *mongodriver.Collection
}

type mongoCollection struct {
	collection *mongodriver.Collection
}

func (c *mongoCollection) FindOne(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) singleResult {
	return c.collection.FindOne(ctx, filter, opts...)
}

func (c *mongoCollection) Find(ctx context.Context, filter interface{}, opts ...*options.FindOptions) (cursor, error) {
	return c.collection.Find(ctx, filter, opts...)
}

func (c *mongoCollection) InsertOne(ctx context.Context, document interface{}, opts ...*options.InsertOneOptions) (*mongodriver.InsertOneResult, error) {
	return c.collection.InsertOne(ctx, document, opts...)
}

func (c *mongoCollection) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongodriver.UpdateResult, error) {
	return c.collection.UpdateOne(ctx, filter, update, opts...)
}

func (c *mongoCollection) DeleteOne(ctx context.Context, filter interface{}, opts ...*options.DeleteOptions) (*mongodriver.DeleteResult, error) {
	return c.collection.DeleteOne(ctx, filter, opts...)
}

func (c *mongoCollection) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	return c.collection.CountDocuments(ctx, filter, opts...)
}

func (c *mongoCollection) Raw() *mongodriver.Collection {
	return c.collection
}

type repositoryConfig[ID any] struct {
	idField     string
	idConverter IDConverter[ID]
	defaultSort interface{}
}

func defaultRepositoryConfig[ID any]() repositoryConfig[ID] {
	return repositoryConfig[ID]{
		idField:     "_id",
		idConverter: func(id ID) (interface{}, error) { return id, nil },
		defaultSort: bson.D{{Key: "_id", Value: 1}},
	}
}

// RepositoryOption customizes MongoRepository behavior.
type RepositoryOption[T any, ID any] func(*repositoryConfig[ID])

// WithIDField sets which BSON field is used as repository identity.
func WithIDField[T any, ID any](field string) RepositoryOption[T, ID] {
	return func(cfg *repositoryConfig[ID]) {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			cfg.idField = trimmed
		}
	}
}

// WithIDConverter sets custom ID translation logic for repository filters.
func WithIDConverter[T any, ID any](converter IDConverter[ID]) RepositoryOption[T, ID] {
	return func(cfg *repositoryConfig[ID]) {
		if converter != nil {
			cfg.idConverter = converter
		}
	}
}

// WithObjectIDHex configures the repository to accept hex-string IDs as ObjectIDs.
func WithObjectIDHex[T any]() RepositoryOption[T, string] {
	return func(cfg *repositoryConfig[string]) {
		cfg.idConverter = ObjectIDHexConverter
	}
}

// WithDefaultSort sets default FindAll sorting. Pass nil to disable default sorting.
func WithDefaultSort[T any, ID any](sort interface{}) RepositoryOption[T, ID] {
	return func(cfg *repositoryConfig[ID]) {
		cfg.defaultSort = sort
	}
}

// MongoRepository implements Repository[T, ID] over a Mongo collection.
// It keeps CRUD semantics simple and exposes Collection() for advanced document workflows.
type MongoRepository[T any, ID any] struct {
	collection  collection
	idField     string
	idConverter IDConverter[ID]
	defaultSort interface{}
}

// Compile-time check for contract compatibility.
var _ contracts.Repository[any, any, httpx.PageQuery, httpx.Page[any]] = (*MongoRepository[any, any])(nil)

// NewRepository creates a repository from a Keel Mongo client and collection name.
func NewRepository[T any, ID any](client *Client, collectionName string, opts ...RepositoryOption[T, ID]) *MongoRepository[T, ID] {
	var coll *mongodriver.Collection
	if client != nil {
		coll = client.Collection(collectionName)
	}
	return NewRepositoryFromCollection[T, ID](coll, opts...)
}

// NewRepositoryFromCollection creates a repository from a raw mongo collection.
func NewRepositoryFromCollection[T any, ID any](collection *mongodriver.Collection, opts ...RepositoryOption[T, ID]) *MongoRepository[T, ID] {
	return newRepository[T, ID](&mongoCollection{collection: collection}, opts...)
}

func newRepository[T any, ID any](coll collection, opts ...RepositoryOption[T, ID]) *MongoRepository[T, ID] {
	cfg := defaultRepositoryConfig[ID]()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return &MongoRepository[T, ID]{
		collection:  coll,
		idField:     cfg.idField,
		idConverter: cfg.idConverter,
		defaultSort: cfg.defaultSort,
	}
}

// Collection exposes the underlying *mongo.Collection for advanced queries.
func (r *MongoRepository[T, ID]) Collection() *mongodriver.Collection {
	if r == nil || r.collection == nil {
		return nil
	}
	return r.collection.Raw()
}

// FindByID returns nil,nil when no document matches.
func (r *MongoRepository[T, ID]) FindByID(ctx context.Context, id ID) (*T, error) {
	if err := r.ensureReady(); err != nil {
		return nil, err
	}

	filter, err := r.idFilter(id)
	if err != nil {
		return nil, err
	}

	var entity T
	if err := r.collection.FindOne(ctx, filter).Decode(&entity); err != nil {
		if errors.Is(err, mongodriver.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &entity, nil
}

// FindAll returns paginated documents from the collection.
func (r *MongoRepository[T, ID]) FindAll(ctx context.Context, q httpx.PageQuery) (httpx.Page[T], error) {
	if err := r.ensureReady(); err != nil {
		return httpx.Page[T]{}, err
	}

	q = normalizePageQuery(q)
	filter := bson.D{}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return httpx.Page[T]{}, err
	}

	findOpts := options.Find().
		SetSkip(int64((q.Page - 1) * q.Limit)).
		SetLimit(int64(q.Limit))
	if r.defaultSort != nil {
		findOpts.SetSort(r.defaultSort)
	}

	cursor, err := r.collection.Find(ctx, filter, findOpts)
	if err != nil {
		return httpx.Page[T]{}, err
	}
	defer cursor.Close(ctx)

	var items []T
	if err := cursor.All(ctx, &items); err != nil {
		return httpx.Page[T]{}, err
	}

	return httpx.NewPage(items, int(total), q.Page, q.Limit), nil
}

// FindOneByFilter runs a direct Mongo find-one query and decodes into T.
func (r *MongoRepository[T, ID]) FindOneByFilter(ctx context.Context, filter interface{}, opts ...*options.FindOneOptions) (*T, error) {
	if err := r.ensureReady(); err != nil {
		return nil, err
	}

	var entity T
	if err := r.collection.FindOne(ctx, normalizeFilter(filter), opts...).Decode(&entity); err != nil {
		if errors.Is(err, mongodriver.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &entity, nil
}

// FindMany runs a direct Mongo find query with custom filter/options.
func (r *MongoRepository[T, ID]) FindMany(ctx context.Context, filter interface{}, opts ...*options.FindOptions) ([]T, error) {
	if err := r.ensureReady(); err != nil {
		return nil, err
	}

	cursor, err := r.collection.Find(ctx, normalizeFilter(filter), opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []T
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// Create inserts one document.
func (r *MongoRepository[T, ID]) Create(ctx context.Context, entity *T) error {
	if err := r.ensureReady(); err != nil {
		return err
	}
	if entity == nil {
		return errors.New("entity is required")
	}

	_, err := r.collection.InsertOne(ctx, entity)
	return err
}

// Update replaces ALL fields of the document (except the ID field) via $set.
// Equivalent to HTTP PUT — every field is overwritten with the entity values.
// Use Patch for partial updates (HTTP PATCH semantics).
func (r *MongoRepository[T, ID]) Update(ctx context.Context, id ID, entity *T) error {
	if err := r.ensureReady(); err != nil {
		return err
	}
	if entity == nil {
		return errors.New("entity is required")
	}

	filter, err := r.idFilter(id)
	if err != nil {
		return err
	}

	doc, err := documentForUpdate(entity, r.idField)
	if err != nil {
		return err
	}

	_, err = r.collection.UpdateOne(ctx, filter, bson.M{"$set": doc})
	return err
}

// Patch applies a partial update using only the fields present in patch (HTTP PATCH semantics).
// patch can be a bson.M, bson.D, or any BSON-marshalable struct — only the keys
// provided in patch are written; all other fields in the document are left unchanged.
func (r *MongoRepository[T, ID]) Patch(ctx context.Context, id ID, patch *any) error {
	if err := r.ensureReady(); err != nil {
		return err
	}
	if patch == nil {
		return errors.New("patch is required")
	}

	filter, err := r.idFilter(id)
	if err != nil {
		return err
	}

	_, err = r.collection.UpdateOne(ctx, filter, bson.M{"$set": patch})
	return err
}

// Delete removes one document by repository ID.
func (r *MongoRepository[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := r.ensureReady(); err != nil {
		return err
	}

	filter, err := r.idFilter(id)
	if err != nil {
		return err
	}

	_, err = r.collection.DeleteOne(ctx, filter)
	return err
}

func (r *MongoRepository[T, ID]) ensureReady() error {
	if r == nil || r.collection == nil || r.collection.Raw() == nil {
		return errors.New("collection is nil")
	}
	return nil
}

func (r *MongoRepository[T, ID]) idFilter(id ID) (bson.M, error) {
	value, err := r.idConverter(id)
	if err != nil {
		return nil, err
	}
	return bson.M{r.idField: value}, nil
}

func documentForUpdate(entity interface{}, idField string) (bson.M, error) {
	payload, err := bson.Marshal(entity)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal entity: %w", err)
	}

	var doc bson.M
	if err := bson.Unmarshal(payload, &doc); err != nil {
		return nil, fmt.Errorf("unable to unmarshal entity as bson map: %w", err)
	}

	delete(doc, idField)
	if len(doc) == 0 {
		return nil, errors.New("update document has no fields")
	}

	return doc, nil
}

func normalizeFilter(filter interface{}) interface{} {
	if filter == nil {
		return bson.D{}
	}
	return filter
}

func normalizePageQuery(q httpx.PageQuery) httpx.PageQuery {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 20
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	return q
}
