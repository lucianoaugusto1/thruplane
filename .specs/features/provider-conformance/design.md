# Native provider conformance design

**Spec:** `.specs/features/provider-conformance/spec.md`

`internal/provider/conformance_test.go` loads one JSON file per native
protocol from `internal/provider/testdata/conformance/`. Each case contains a
public Chat Completions request, the expected upstream request, a simulated
provider response, and the expected normalized result. JSON comparisons are
semantic so key order cannot create false failures. Response expectations are
subsets because normalized responses include a runtime `created` timestamp.

The harness uses a fresh local `httptest.Server` for every case. It fixes the
Bedrock signing clock and verifies the payload hash so SigV4 remains
deterministic without coupling fixtures to a random test-server port. Error
fixtures can assert that no upstream call occurs.

`tests/provider-live` contains a build-tagged test runner. The normal package
has no network tests. `go test -tags=live ./tests/provider-live` requires
explicit environment configuration and runs only the selected provider and
scenarios. It uses small inline media samples and a low, explicit token limit;
it never logs credentials or request bodies.

The validation document reports fixture coverage separately from live
evidence. A green local fixture means the gateway translation matches the
recorded contract shape; it does not prove current provider acceptance.
