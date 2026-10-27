# Goxide std

Shared packages used by Goxide programs and generated Go code. They require Go 1.27. The runtime packages use the Go standard library; the procedural macro package uses the public Goxide syntax SDK.

| Package | Purpose |
| --- | --- |
| `std` | Ordinary `Default`, `Clone`, `Debug`, `Display`, `PartialEq`, `Eq` and `NativeKey` traits, with explicit primitive and container implementations. Reexports `From` and `Into`. |
| `std::prelude` | Common traits, `default_value`, derive, printing and enum assertion macros, and the `derive` namespace. Automatically available in each `.gox` file. |
| `std::derive` | Procedural `Default`, `Clone`, `Debug`, `PartialEq`, `Eq`, `NativeKey` and `Error` derives. |
| `std::convert` | Goxide source traits `From[S]` and `Into[T]`, with ordinary identity and From-to-Into implementations. |
| `github.com/goxide-lang/std/sum` | `Option` and `Result`, constructors, discriminants, checked payload reads and projections. |
| `github.com/goxide-lang/std/formatting` | Scalar formatting and checked writer output for `print!` / `println!`. Failed or short writes panic with `*WriteError`, preserving the cause and byte counts. |
| `github.com/goxide-lang/std/runtime` | Assignment-propagation guards and the `Comparable` constraint used by generated code. The Go package name is `goxideruntime`. |
| `github.com/goxide-lang/std/typeinfo` | Structural `CarrierFor[T, S]` and `Of[T, S]` type-carrier helpers. `Of` returns the zero carrier without constructing `T`, invoking a method, or using reflection. |

Generated programs import only the packages they use; they do not need the compiler module. Serialization protocols and JSON live in the separate [serde repository](https://github.com/goxide-lang/serde).

`std::convert` is a Goxide source package described by `goxide.toml`. The compiler builds its ordinary trait implementations alongside the selected source dependency graph. `From` preserves ordinary value and reference semantics; it does not clone, unwrap interfaces, or normalize typed nil. `Into` calls the selected `From` through its declared body. Error propagation uses the canonical `From` only when direct assignment is unavailable.

## Use with Goxide

Goxide v0.0.3 pins std v0.0.2. Use that std version in the project's dependency declaration. Each `.gox` file automatically receives `std::prelude` from the selected std source:

```goxide
#[derive(Default, Clone, Debug)]
struct Config { Port int; Name string }

func example() {
    value := default_value[Config]()
    copy := Clone::clone(&value)
    println!("{:?}", copy)
}
```

The prelude exports the traits `Default`, `Clone`, `Debug`, `Display`, `PartialEq`, `Eq`, `NativeKey`, `From`, and `Into`; the function `default_value`; the macros `print!`, `println!`, and `enum_assert!`; and the derive macros `Default`, `Clone`, `Debug`, `PartialEq`, `Eq`, `NativeKey`, and `Error`. The `derive` namespace remains available for qualified paths such as `derive::Debug`. `Option`, `Result`, `Some`, `None`, `Ok`, and `Err` are already supplied by the language.

Macros occupy a separate namespace from ordinary types, traits, values, and modules. `Debug` in a trait bound selects the trait; `Debug` in `#[derive(Debug)]` selects the macro. A single `use library::Name;` imports every public `Name` exported by that target in both namespaces. Function-like, attribute, and derive calls still require the matching macro entry kind.

Within each namespace, declarations and named `use` bindings take precedence over explicit globs, which take precedence over the automatic prelude. A binding in one namespace does not hide a name in the other; ambiguity within one namespace remains an error. Private `use` bindings retain their existing file scope. Macro expansion retains definition-site hygiene. Generated Go imports follow actual symbol use; the prelude does not add an import for every exported name. Compile-time-only macros add no runtime import to the consuming file. Native `gen` retains their actual generation dependencies in its tools file; an unused prelude adds no tools import.

`Default` is an ordinary constructor protocol with the static method `Default() Self`. Zero initialization does not call it. An enum derives Default from exactly one `#[default]` variant and only requires defaults for that variant's payload. `Option[T]` defaults to `None`; slices, arrays, maps and `Result` have no Default implementation.

```goxide
struct Settings { Port int }
impl Default for Settings {
    func Default() Self { return Self{Port: 8080} }
}
```

`Clone` copies supported containers into independent storage, retaining the difference between nil and empty slices or maps. `Debug` formats fields in declaration order and sorts string map keys. Printing and deriving use the same Debug protocol, including complex values and fields. `PartialEq` compares payloads through their explicit implementations, short circuits in field order, and distinguishes nil from empty containers. Floating-point comparison keeps native NaN behavior.

`Eq` is an ordinary marker promise. Deriving it requires `DerivedPartialEq` for the target and `Eq` for its fields. The PartialEq derive supplies `DerivedPartialEq`; authors can also implement that marker explicitly. No compiler provenance check makes this promise unforgeable.

Map equality additionally requires `NativeKey`, which promises that the key's PartialEq agrees with stable native Go map-key equality. An Eq implementation does not imply NativeKey. Struct keys opt in with `#[derive(std::derive::NativeKey)]` or an explicit implementation; the derive requires NativeKey for every field. Builtin boolean, integer and string keys, and arrays of native keys, have library implementations. Float, complex and enum keys have no derived NativeKey implementation. Map Clone retains its exact builtin scalar-key boundary, while map Debug accepts string keys.

The `print!` and `println!` macros accept `{}` for Display, `{:?}` for Debug, and escaped `{{` and `}}`. Every argument is evaluated once, before formatting starts. The macros retain the caller's control-flow frame and write the resulting text once through `formatting.Write`.

`Error` uses `#[error("message {field}")]`, `#[source]` and `#[from]` to generate ordinary `Error`/`Unwrap` methods. A `#[from]` source also generates a `From[error]` implementation. Message fields must have exact builtin scalar types; source fields must have the exact `error` type, including transparent aliases.

Run generation again after changing the compiler or this dependency. See [using Goxide in an existing Go project](https://github.com/goxide-lang/goxide/blob/main/docs/using-goxide-gen.md).

## Option and Result representation

`sum.Option[T]` and `sum.Result[T, E]` are Go structs with private tags and separate inline payload slots (`GoxideSumABI = 3`). Option's zero value is `None`; Result's zero value is invalid. `None` and `Some(nil)` are distinct. Assignment and pointer, slice or map payloads retain ordinary Go shallow-copy semantics; cloning is explicit.

User-defined Goxide enums use a sealed interface and one pointer struct per variant. They reject nil values. These user enums and std's tagged structs currently have different representations; the compiler lowers their matches accordingly. Unifying std's representation is tracked with safe std in [MVP-6](https://github.com/goxide-lang/goxide/blob/main/docs/mvp-gap-audit.md).

Goxide enum variants and payload slots are immutable. Assigning a new enum value rebinds the variable. A slice, map or pointer payload read by value retains its own permissions, so its referenced contents can still be mutable without replacing the enum's payload slot.

Use `None`, `NewSome`, `NewOk` and `NewErr` to construct values. `TagOption` / `TagResult` read raw tags; `ValidateOption` / `ValidateResult` check their validity. `ValueSome` / `ValueOk` / `ValueErr` return payload copies. `ProjectSome` / `ProjectOk` / `ProjectErr` check the active variant and return a pointer to its actual slot.

A saved projection remains attached to that slot if the enclosing value is overwritten. Different variants have separate slots; inactive slots do not affect validation. Projections provide ordinary Go pointers, including ordinary aliasing and concurrency rules.

## Development

```sh
GOWORK=off GOTOOLCHAIN=local go test -race ./...
GOWORK=off GOTOOLCHAIN=local go vet ./...
```

Licensed under [Mozilla Public License 2.0](LICENSE).
