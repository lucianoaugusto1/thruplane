# Contributing to NexoRoute

Thanks for helping improve NexoRoute Community.

## Before you start

Use a public issue or discussion to align on large features before investing in
an implementation. Small bug fixes and documentation corrections can go
directly to a pull request once the repository has a public remote.

Do not use a public issue for a suspected vulnerability. Follow
[SECURITY.md](SECURITY.md) instead.

## Development workflow

1. Create a focused branch from the default branch.
2. Add or update tests before changing executable behavior.
3. Keep the change limited to one concern.
4. Run the project gate.

   ```sh
   gofmt -w ./cmd ./internal
   go test -race ./...
   go vet ./...
   go build ./cmd/nexoroute
   ```

5. Update README or configuration examples when behavior changes.
6. Use a Conventional Commit message when practical.

## Pull request expectations

A change is ready for review when it:

- Explains the user or operator problem
- Includes deterministic tests for new behavior
- Preserves streaming and request cancellation
- Avoids logging prompts, provider credentials, or authorization headers
- Keeps provider-specific transformations outside the public API contract
- Passes the complete project gate without skipped tests

## Scope and compatibility

NexoRoute exposes an OpenAI-compatible subset. Do not claim compatibility for a
field or endpoint until tests cover it against the gateway contract. Preserve
unknown JSON fields whenever the gateway does not need to interpret them.

Breaking changes need an explicit migration note and a `!` Conventional Commit
marker.

## Licensing

Unless stated otherwise, contributions submitted for inclusion in NexoRoute
Community are licensed under Apache License 2.0, as described in `LICENSE`.
