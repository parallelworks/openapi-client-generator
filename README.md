# openapi-client-generator

A Go code generator that produces feature-rich HTTP client packages from OpenAPI 3.1 specifications.

Given any OpenAPI 3.1 (or 3.0) spec, it outputs a complete, idiomatic Go client package with zero external dependencies.

## Features

- **Typed models** — structs, enums, type aliases, and union types (allOf/oneOf/anyOf) with JSON marshaling
- **Client methods** — per-operation methods with `context.Context`, typed parameters, and typed responses
- **Authentication** — `AuthProvider` interface with built-in Bearer, API key, and Basic auth
- **Error handling** — `APIError` with sentinel errors (`errors.Is`), typed error wrappers with parsed response bodies (`errors.As`)
- **Pagination** — auto-detected cursor/offset pagination with generic `PageIterator[T]`
- **Retries** — configurable exponential backoff with jitter and `Retry-After` header support
- **Middleware** — composable request/response middleware chain
- **OpenAPI 3.1** — full support for JSON Schema 2020-12, nullable type arrays, `$ref` resolution

## Installation

Add as a tool dependency to your project (recommended):

```sh
go get -tool github.com/parallelworks/openapi-client-generator@latest
```

This pins the generator version in your `go.mod` so every team member uses the same version.

Or install globally:

```sh
go install github.com/parallelworks/openapi-client-generator@latest
```

## Usage

```sh
go tool openapi-client-generator generate --spec petstore.yaml --out ./gen/petstore
```

If installed globally via `go install`:

```sh
openapi-client-generator generate --spec petstore.yaml --out ./gen/petstore
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--spec` | `-s` | Path to OpenAPI spec file (required) |
| `--out` | `-o` | Output directory for generated code (required) |
| `--package` | `-p` | Go package name (default: derived from output dir) |
| `--user-agent` | | Default `User-Agent` for generated clients (default `openapi-client-generator/1.0`) |
| `--allow-remote-refs` | | Allow fetching remote `$ref` targets |


## Generated Code

For a Petstore spec, the generator produces:

```
gen/petstore/
├── auth.go          # AuthProvider interface, BearerAuth, APIKeyAuth, BasicAuth
├── client.go        # Client struct, NewClient(), do() with retry + middleware
├── errors.go        # APIError, sentinel errors, typed ErrorResponse wrappers
├── helpers.go       # URL building, query param encoding
├── middleware.go     # Middleware type, WithMiddleware()
├── operations.go    # ListPets(), CreatePet(), GetPetByID(), DeletePet()
├── options.go       # WithHTTPClient(), WithUserAgent()
├── pagination.go    # PageIterator[T], ListPetsIter()
├── retry.go         # RetryConfig, WithRetry(), WithDefaultRetry()
└── types.go         # Pet, PetStatus, PetList, CreatePetRequest, Error
```

### Example usage of the generated client

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"

    "example.com/gen/petstore"
)

func main() {
    client := petstore.NewClient(
        "https://petstore.example.com/v1",
        petstore.WithAuth(&petstore.BearerAuth{Token: "my-token"}),
        petstore.WithDefaultRetry(),
    )

    ctx := context.Background()

    // List pets with pagination
    iter := client.ListPetsIter(ctx)
    err := iter.ForEach(func(pet petstore.Pet) error {
        fmt.Printf("Pet: %s (ID: %d)\n", pet.Name, pet.ID)
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create a pet
    err = client.CreatePet(ctx, petstore.CreatePetRequest{Name: "Buddy"})
    if err != nil {
        log.Fatal(err)
    }

    // Get a specific pet — check for a known status code
    pet, err := client.GetPetByID(ctx, 123)
    if errors.Is(err, petstore.ErrNotFound) {
        log.Println("Pet not found")
        return
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found: %s\n", pet.Name)

    // Extract the parsed error body using the typed error wrapper
    _, err = client.GetPetByID(ctx, 999)
    var errResp *petstore.ErrorResponse
    if errors.As(err, &errResp) {
        fmt.Printf("Error %d: %s\n", errResp.Detail.Code, errResp.Detail.Message)
    }
}
```

### Discriminated unions

A `oneOf`/`anyOf` with a `discriminator` generates a wrapper whose `Value` holds
the decoded variant. A discriminator value the client does not know is **not** an
error: `Value` stays nil, the original JSON is kept, and re-marshaling returns it
unchanged. Adding a variant server-side therefore stays backward compatible, and
one unrecognized element does not fail the payload it appears in.

```go
for _, shape := range shapes {
    if shape.IsUnknownVariant() {
        log.Printf("skipping unsupported shape %q", shape.UnknownDiscriminator())
        continue // shape.Raw() still holds the original JSON
    }
    switch v := shape.Value.(type) {
    case petstore.Circle:
        ...
    }
}
```

When every variant composes the same schema through `allOf`, the wrapper also
exposes it, so the fields all variants share are readable without a type switch
that has to be revisited whenever a variant is added:

```go
for _, pet := range pets {
    if base := pet.Base(); base != nil {
        fmt.Println(base.ID, base.Name)
    }
}
```

`Base()` returns nil for an unknown variant, and is generated only when *every*
variant composes the *same* single base.

A payload that carries no discriminator property at all is still an error — there
is nothing to identify it by — as is a union *without* a discriminator when no
variant matches. When the schema declares a `discriminator` but no `mapping`, the
variant's schema name is used as the discriminator value, per the OpenAPI spec.

## License

[MIT](LICENSE) — Copyright (c) 2026 Parallel Works
