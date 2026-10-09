# goxide std — candidate

Core types shared by Goxide-generated Go packages. This unreleased candidate is not a v1.0 stability promise. It requires a compiler implementing the matching sum ABI.

`formatting` supplies scalar formatters and checked stdout/writer output for generated `print!` / `println!` calls. Format-string parsing and protocol selection remain compiler responsibilities. A failed or short write panics with `*formatting.WriteError`, retaining the original cause and byte counts.

`runtime` (Go package name `goxideruntime`) supplies the assignment propagation guard and the hygienic `Comparable` constraint used by generated Go. These are compiler lowering details, not source-language protocols or a stable public ABI.

All packages depend only on the Go standard library. Generated programs import `github.com/goxide-lang/std/formatting`, `github.com/goxide-lang/std/runtime`, and `github.com/goxide-lang/std/sum` as needed; they do not require the compiler module. The former compiler-owned formatting/runtime paths have been removed, with no forwarding packages. Regenerate existing output with the matching compiler.

`sum` provides Option/Result as values with private tags and separate inline payload slots. Assignment has ordinary Go shallow-copy semantics. Cloning is explicit. Option's zero value is None; Result's zero value is invalid. A shape check does not validate application-specific payloads; generated callers must retain their registered payload checks.

`TagOption` / `TagResult` read raw discriminants without validating them, allowing callers to choose their invalid-value error policy; they do not replace `ValidateOption` / `ValidateResult`. `ValueSome` / `ValueOk` / `ValueErr` copy its payload, while `ProjectSome` / `ProjectOk` / `ProjectErr` return an ordinary Go pointer to its actual slot after checking the variant. A saved pointer remains attached to that slot when the containing value is overwritten. Constructors clear unused slots, but writes through old pointers can subsequently fill inactive slots; validation and language protocols must ignore those inactive values. Distinct variants never share a slot. Slice/map/pointer payloads retain ordinary shallow-copy behavior. Retained pointers or inactive payloads may keep allocations alive; these APIs promise neither exclusive access nor race freedom.

The format-independent serialization protocol and JSON implementation belong to the separate `github.com/goxide-lang/serde` module, not this module. No experimental API is included.

The existing Mozilla Public License 2.0 text is retained in LICENSE. No release tag is provided by this candidate.

Run `go test -race ./...` and `go vet ./...` with Go 1.27 or newer.

Representation migration: `GoxideSumABI = 3` replaces ABI 2's interfaces and exported `Some` / `Ok` / `Err` case structs. There is no old-layout compatibility API. Use the constructors, raw tag readers, and checked value/projection functions, and regenerate all producers, consumers and compiler metadata together. Option is no longer a nil interface and cannot be compared with nil.
