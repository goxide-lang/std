# Goxide std

Shared Go packages used by Goxide-generated code. They require Go 1.27 and depend only on the Go standard library.

| Package | Purpose |
| --- | --- |
| `github.com/goxide-lang/std/sum` | `Option` and `Result`, constructors, discriminants, checked payload reads and projections. |
| `github.com/goxide-lang/std/formatting` | Scalar formatting and checked writer output for `print!` / `println!`. Failed or short writes panic with `*WriteError`, preserving the cause and byte counts. |
| `github.com/goxide-lang/std/runtime` | Assignment-propagation guards and the `Comparable` constraint used by generated code. The Go package name is `goxideruntime`. |
| `github.com/goxide-lang/std/typeinfo` | Structural `CarrierFor[T, S]` and `Of[T, S]` type-carrier helpers. `Of` returns the zero carrier without constructing `T`, invoking a method, or using reflection. |

Generated programs import only the packages they use; they do not need the compiler module. Serialization protocols and JSON live in the separate [serde repository](https://github.com/goxide-lang/serde).

## Use with Goxide

Use the std version pinned by your compiler's `go.mod`. After std v0.0.1 is published, install it with:

```sh
go get github.com/goxide-lang/std@v0.0.1
```

Run generation again after changing the compiler or this dependency. See [using Goxide in an existing Go project](https://github.com/goxide-lang/goxide/blob/main/docs/using-goxide-gen.md).

## Option and Result representation

`sum.Option[T]` and `sum.Result[T, E]` are Go structs with private tags and separate inline payload slots (`GoxideSumABI = 3`). Option's zero value is `None`; Result's zero value is invalid. `None` and `Some(nil)` are distinct. Assignment and pointer, slice or map payloads retain ordinary Go shallow-copy semantics; cloning is explicit.

User-defined Goxide enums use a sealed interface and one pointer struct per variant. They reject nil values. These user enums and std's tagged structs currently have different representations; the compiler lowers their matches accordingly. Unifying std's representation is tracked with safe std in [MVP-6](https://github.com/goxide-lang/goxide/blob/main/docs/mvp-gap-audit.md).

Use `None`, `NewSome`, `NewOk` and `NewErr` to construct values. `TagOption` / `TagResult` read raw tags; `ValidateOption` / `ValidateResult` check their validity. `ValueSome` / `ValueOk` / `ValueErr` return payload copies. `ProjectSome` / `ProjectOk` / `ProjectErr` check the active variant and return a pointer to its actual slot.

A saved projection remains attached to that slot if the enclosing value is overwritten. Different variants have separate slots; inactive slots do not affect validation. Projections provide ordinary Go pointers, including ordinary aliasing and concurrency rules.

## Development

```sh
GOWORK=off GOTOOLCHAIN=local go test -race ./...
GOWORK=off GOTOOLCHAIN=local go vet ./...
```

Licensed under [Mozilla Public License 2.0](LICENSE).
