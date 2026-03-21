<img src="https://cdn.slicesoft.dev/boat.svg" width="400" />

# ss-keel-mongo
Official MongoDB addon for Keel — document-first persistence via the official Go driver.

[![CI](https://github.com/slice-soft/ss-keel-mongo/actions/workflows/ci.yml/badge.svg)](https://github.com/slice-soft/ss-keel-mongo/actions)
[![Release](https://img.shields.io/github/v/release/slice-soft/ss-keel-mongo)](https://github.com/slice-soft/ss-keel-mongo/releases)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
[![Go Report Card](https://goreportcard.com/badge/github.com/slice-soft/ss-keel-mongo)](https://goreportcard.com/report/github.com/slice-soft/ss-keel-mongo)
[![Go Reference](https://pkg.go.dev/badge/github.com/slice-soft/ss-keel-mongo.svg)](https://pkg.go.dev/github.com/slice-soft/ss-keel-mongo)
![License](https://img.shields.io/badge/License-MIT-green)
![Made in Colombia](https://img.shields.io/badge/Made%20in-Colombia-FCD116?labelColor=003893)


## MongoDB addon for Keel

`ss-keel-mongo` is the official Keel addon for MongoDB using the official Go driver.
It keeps the same high-level repository contract used in Keel, but with Mongo-native behavior:
collection-first workflows, document filters, nested fields, and optional schema flexibility.

---

## Installation

```bash
keel add mongo
```

The Keel CLI will:
1. Add `github.com/slice-soft/ss-keel-mongo` to your project.
2. Add `MONGO_URI` and `MONGO_DATABASE` examples to `.env`.
3. Wire Mongo bootstrap and health registration in `cmd/main.go`.

Manual install:

```bash
go get github.com/slice-soft/ss-keel-mongo
```

---

## Configuration

```go
import (
    "github.com/slice-soft/ss-keel-core/config"
    "github.com/slice-soft/ss-keel-mongo/mongo"
)

client, err := mongo.New(mongo.Config{
    URI:      config.GetEnvOrDefault("MONGO_URI", "mongodb://localhost:27017"),
    Database: config.GetEnvOrDefault("MONGO_DATABASE", "app"),
    Logger:   appLogger,
})
if err != nil {
    appLogger.Error("failed to start app: %v", err)
}
defer client.Close()
```

Defaults applied when omitted:

| Setting | Default |
|---|---|
| `URI` | `mongodb://localhost:27017` |
| `ConnectTimeout` | 10s |
| `PingTimeout` | 2s |
| `DisconnectTimeout` | 5s |
| `ServerSelectionTimeout` | 5s |
| `MaxPoolSize` | 25 |
| `MaxConnIdleTime` | 15m |

---

## Generic repository

`MongoRepository[T, ID]` implements Keel's CRUD contract and exposes Mongo-native extension points.

```go
type User struct {
    ID      string             `bson:"_id,omitempty"`
    Name    string             `bson:"name"`
    Profile struct {
        Country string `bson:"country"`
    } `bson:"profile,omitempty"`
}

repo := mongo.NewRepository[User, string](client, "users")
```

Standard methods:

- `FindByID(ctx, id)`
- `FindAll(ctx, mongo.PageQuery{Page: 1, Limit: 20})`
- `Create(ctx, &entity)`
- `Update(ctx, id, &entity)`
- `Patch(ctx, id, &entity)`
- `Delete(ctx, id)`

Mongo-focused methods:

- `FindOneByFilter(ctx, filter)`
- `FindMany(ctx, filter, options...)`
- `Collection()` for raw driver access when you need full Mongo power.

---

## ID strategies

By default, IDs are used as-is against `_id`. This matches Keel's generated code, which uses UUID strings across every persistence addon.

Custom IDs and fields are also supported:

```go
repo := mongo.NewRepository[User, string](
    client,
    "users",
    mongo.WithIDField[User, string]("slug"),
    mongo.WithIDConverter[User, string](func(id string) (interface{}, error) {
        return strings.ToLower(id), nil
    }),
)
```

---

## EntityBase

`ss-keel-mongo` ships a ready-made `EntityBase` struct you can embed in any document entity to get `ID`, `CreatedAt`, and `UpdatedAt` with the correct BSON tags pre-configured:

```go
type EntityBase struct {
    ID        string `json:"id"         bson:"_id,omitempty"`
    CreatedAt int64  `json:"created_at" bson:"created_at,omitempty"`
    UpdatedAt int64  `json:"updated_at" bson:"updated_at,omitempty"`
}
```

`CreatedAt` and `UpdatedAt` store Unix **milliseconds**. Call the two helpers to stamp timestamps — the generated repository does this automatically:

```go
entity.OnCreate() // sets CreatedAt and UpdatedAt — call before inserting
entity.OnUpdate() // sets only UpdatedAt — call before updating
```

```go
type ProductEntity struct {
    mongo.EntityBase
    Name  string  `bson:"name"`
    Price float64 `bson:"price"`
}
```

---

## Health checker

Register Mongo in Keel's `/health` endpoint:

```go
app.RegisterHealthChecker(mongo.NewHealthChecker(client))
```

Response includes:

```json
{ "mongodb": "UP" }
```

---

## How it differs from `ss-keel-gorm`

- `ss-keel-gorm` is SQL-first and relational.
- `ss-keel-mongo` is document-first and filter-first.
- `Update` in Mongo applies `$set` to non-ID fields; it does not force relational persistence patterns.
- Advanced document queries are expected through `FindMany`, `FindOneByFilter`, or `Collection()`.

---

## Current limitations

- No automatic migration layer (Mongo schema is application-driven).
- No transaction helper abstraction yet (you can use driver sessions directly through `Collection()` / `Native()`).
- `FindAll` currently paginates over the whole collection; use custom filters for domain-specific listings.

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for setup and repository-specific rules.
The base workflow, commit conventions, and community standards live in [ss-community](https://github.com/slice-soft/ss-community/blob/main/CONTRIBUTING.md).

## Community

| Document | |
|---|---|
| [CONTRIBUTING.md](https://github.com/slice-soft/ss-community/blob/main/CONTRIBUTING.md) | Workflow, commit conventions, and PR guidelines |
| [GOVERNANCE.md](https://github.com/slice-soft/ss-community/blob/main/GOVERNANCE.md) | Decision-making, roles, and release process |
| [CODE_OF_CONDUCT.md](https://github.com/slice-soft/ss-community/blob/main/CODE_OF_CONDUCT.md) | Community standards |
| [VERSIONING.md](https://github.com/slice-soft/ss-community/blob/main/VERSIONING.md) | SemVer policy and breaking changes |
| [SECURITY.md](https://github.com/slice-soft/ss-community/blob/main/SECURITY.md) | How to report vulnerabilities |
| [MAINTAINERS.md](https://github.com/slice-soft/ss-community/blob/main/MAINTAINERS.md) | Active maintainers |

## License

MIT License - see [LICENSE](LICENSE) for details.

## Links

- Website: [keel-go.dev](https://keel-go.dev)
- GitHub: [github.com/slice-soft/ss-keel-mongo](https://github.com/slice-soft/ss-keel-mongo)
- Documentation: [docs.keel-go.dev](https://docs.keel-go.dev)

---

Made by [SliceSoft](https://slicesoft.dev) - Colombia
