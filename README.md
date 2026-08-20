# openapi-client-generator

A Go code generator that produces feature-rich HTTP client packages from OpenAPI 3.1 specifications.

Given any OpenAPI 3.1 (or 3.0) spec, it outputs a complete, idiomatic Go client package with zero external dependencies.

## Features

- **Typed models** — structs, enums, type aliases, and union types (allOf/oneOf/anyOf) with JSON marshaling
- **Client methods** — per-operation methods with `context.Context`, typed parameters, and typed responses
- **Webhooks and callbacks**: typed payloads and a dispatcher for the requests the API sends you
- **Server URLs**: `DefaultBaseURL` from the spec, with a builder for templated servers
- **Response headers**: status and headers captured through the context, with declared headers parsed per operation
- **Authentication** — `AuthProvider` interface with built-in Bearer, API key, and Basic auth
- **Error handling** — `APIError` with sentinel errors (`errors.Is`), typed error wrappers with parsed response bodies (`errors.As`), readable messages via `x-ms-primary-error-message`
- **Pagination**: auto-detected cursor, offset, and page pagination with a generic `PageIterator[T]`
- **Retries**: configurable exponential backoff with jitter, honoring `Retry-After` in both forms and declining a wait past `MaxDelay`
- **Middleware** — composable request/response middleware chain
- **OpenAPI 3.1**: JSON Schema 2020-12, nullable type arrays, `$ref` resolution ([what is not generated](#not-supported))

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

### Server URLs

When the spec declares a server, the generated package states it, so the URL does
not have to be copied into the code:

```go
client := petstore.NewClient(petstore.DefaultBaseURL)
```

`DefaultBaseURL` is the first server the spec lists, with every template variable
at its default. A templated server also gets a builder, one parameter per
variable in the order the URL uses them, where an empty argument takes that
variable's default:

```yaml
servers:
  - url: https://{region}.api.example.com/{basePath}
    variables:
      region:   { default: us-east-1, enum: [us-east-1, eu-west-1] }
      basePath: { default: v2 }
```

```go
client := petstore.NewClient(petstore.ServerURL("eu-west-1", ""))
// https://eu-west-1.api.example.com/v2
```

A relative server URL (`/api/v3`) gets neither, since it resolves against
wherever the spec is served and the generated package cannot know that host.
### Response headers

A method returns the decoded body, so what a response says outside its body is
read through a capture on the context:

```go
ctx, meta := petstore.WithResponseCapture(ctx)
pet, err := client.CreatePet(ctx, newPet)

meta.StatusCode                        // 201
meta.Header.Get("X-Trace-Id")          // any header, declared or not
```

Headers the spec declares are also available parsed, per operation:

```go
h := meta.CreatePetHeaders()
h.Location                             // string
h.XRateLimitRemaining                  // *int64, nil when absent
```

A header value arrives as text, so only the kinds text parses into unambiguously
are typed: integers, numbers, and booleans, each a pointer so an absent header is
not a zero that reads as a real value. Everything else, HTTP dates and lists
included, stays the raw string. Error responses are captured too, which is where
a rate limit usually arrives. Calls made with the returned context each overwrite
the meta, so give one capture to one call.

### Union and object parameters

A parameter whose schema is a union with a genuine choice names a type, so the
values it accepts are visible in Go rather than hidden behind `any`:

```yaml
- name: either
  in: query
  schema:
    anyOf: [{ type: string }, { type: integer }]
```

```go
either := petstore.ListItemsEither{Value: 42}
client.ListItems(ctx, petstore.ListItemsParams{Either: &either})
// ?either=42
```

The value the union carries is what goes on the wire, under the parameter's own
style, so a list variant still explodes and a scalar still goes out as itself. A
union carrying nothing sends no parameter at all. One shape is one type, so two
parameters declaring the same union share it.

`anyOf: [string, null]` is not a choice of that kind and still collapses to
`*string`, as do variants that refine a single Go type. Objects written inline in
a parameter are named the same way, and keep encoding under their style.

### Error messages

`Error()` on a typed error wrapper renders the property that carries the
human-readable message, falling back to the raw body when it finds none.

A schema names that property in one of two ways. Marking it is exact, either with
the vendor-neutral `x-error-message` or with Kiota's `x-ms-primary-error-message`:

```yaml
components:
  schemas:
    ErrorResponse:
      type: object
      properties:
        code:
          type: integer
        message:
          type: string
          x-error-message: true
```

`Error()` then renders that property instead of the body:

```
API error 404 Not Found: Pet not found
```

The marked property has to be a string, and the first one a schema marks is the
one used. When it is empty, or the body does not parse, the output falls back to
the raw body, so a message never disappears. `Detail` still holds the whole
parsed body either way.

A schema that marks nothing is read by the names error bodies conventionally use,
most specific first: `message`, `detail`, `error_description`, `title`, `error`.
A body carrying several is tried in that order at runtime, so an RFC 7807
response renders `detail` when it has one and `title` when it does not. A body
with no such property, or with one holding something other than text, keeps the
raw output.

### Webhooks and callbacks

A spec's inbound half, `webhooks` and an operation's `callbacks`, describes
requests the API sends to you. The generated package names their payload types
and decodes them:

```go
// one webhook, its declared type, no assertion
pet, err := petstore.ParsePetCreatedWebhook(body)

// or dispatch on the name your router saw
payload, err := petstore.ParseWebhook(name, body)
switch p := payload.(type) {
case petstore.Pet:
    ...
}
```

`WebhookNames` lists what the spec declares, and a name outside it is an error
rather than a nil payload. Callbacks work the same way through `ParseCallback`
and `CallbackNames`, keyed as `operation.callback`, since a callback arrives at a
URL you registered rather than under a name of its own.

Receiving is yours: the generator produces no HTTP handler and does not verify
signatures, which are vendor specific and not described by a spec. A webhook with
no JSON body to decode gets no parse function, and says so at generation time.

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

When every variant carries the same properties, the wrapper also exposes them, so
the fields all variants share are readable without a type switch that has to be
revisited whenever a variant is added:

```go
for _, pet := range pets {
    if base := pet.Base(); base != nil {
        fmt.Println(base.ID, base.Name)
    }
}
```

`Base()` returns nil for an unknown variant. Variants that compose a shared schema
through `allOf` name it directly, and there has to be exactly one such schema.
Variants that inline the same properties instead, which is all some producers
emit, get a `<Union>Base` struct synthesized from the properties every variant
declares identically: same name, same type, same required-ness. That type is
derived from the variants rather than declared by the spec, so it changes when
they do. The discriminator
is left out, since it is how the variants differ and a spec that spells the base
out keeps it out of the shared schema too. The result is a copy, so writing to it
does not change the variant the union holds.

A payload that carries no discriminator property at all is still an error — there
is nothing to identify it by — as is a union *without* a discriminator when no
variant matches. When the schema declares a `discriminator` but no `mapping`, the
variant's schema name is used as the discriminator value, per the OpenAPI spec.

## Not supported

Constructs the generator reads and does not act on. Each one warns at generation
time rather than passing silently:

| Construct | Behavior |
|---|---|
| `prefixItems` | The array stays a slice of one element type. A tuple has no Go shape a slice can hold. |
| `dependentSchemas` | Not enforced. A property whose shape depends on another is a validation rule, not a type. |
| `patternProperties` with several patterns | The map takes an `any` value type, since the patterns disagree about what a key holds. One pattern types the map. |
| `links` | Read and not used. Following a link is a decision for the caller, not a generated method. |
| `mutualTLS` security scheme | No auth provider. The certificate is configured on the `http.Client`. |

## License

[MIT](LICENSE) — Copyright (c) 2026 Parallel Works
