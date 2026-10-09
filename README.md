# goxide std — candidate

Core types shared by Goxide-generated Go packages. This unreleased candidate is not a v1.0 stability promise. It requires a compiler implementing the matching sum ABI.

`formatting` supplies scalar formatters and checked stdout/writer output for generated `print!` / `println!` calls. Format-string parsing and protocol selection remain compiler responsibilities. A failed or short write panics with `*formatting.WriteError`, retaining the original cause and byte counts.

`runtime` (Go package name `goxideruntime`) supplies the assignment propagation guard and the hygienic `Comparable` constraint used by generated Go. These are compiler lowering details, not source-language protocols or a stable public ABI.

All packages depend only on the Go standard library. Generated programs import `github.com/goxide-lang/std/formatting`, `github.com/goxide-lang/std/runtime`, and `github.com/goxide-lang/std/sum` as needed; they do not require the compiler module. The former compiler-owned formatting/runtime paths have been removed, with no forwarding packages. Regenerate existing output with the matching compiler.

`sum` provides Option/Result value cases, constructors and shape checks. Assignment has ordinary Go shallow-copy semantics. Cloning is explicit. A shape check does not validate application-specific payloads; generated callers must retain their registered payload checks.

The format-independent serialization protocol and JSON implementation belong to the separate `github.com/goxide-lang/serde` module, not this module. No experimental API is included.

The existing Mozilla Public License 2.0 text is retained in LICENSE. No release tag is provided by this candidate.

Run `go test -race ./...` and `go vet ./...` with Go 1.27 or newer.

Naming migration: `GoxideSumABI = 2` replaces the pre-1.0 `HgoSumABI = 1` contract. Regenerate all producers and consumers together with the Goxide naming compiler; module path and Option/Result APIs are unchanged.
