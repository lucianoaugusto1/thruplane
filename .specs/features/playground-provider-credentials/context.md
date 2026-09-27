# Playground provider credentials context

**Date:** September 26, 2026
**Decision owner:** Product

## Product decision

The playground will accept provider credentials for interactive testing. This
is distinct from Thruplane virtual keys: the user selected provider credential
testing as the first credential feature.

The intended interaction follows the useful part of LiteLLM's provider setup
flow: select a provider, enter a physical model and credential fields, test the
connection, and then use that tested setup in the playground conversation.

## Locked decisions

- Provider credentials are transient. They live in browser memory and in the
  active HTTP request only.
- The gateway does not save credentials, mutate its YAML configuration, create
  accounts, or introduce a database.
- Credential testing is separately opt-in and disabled by default.
- The credential endpoint uses the existing inbound gateway bearer key.
- Custom upstream base URLs must match a server-side allowlist.
- Testing a connection makes a real provider request and may incur cost.
- Existing configured aliases remain the default playground mode.

## Implementation discretion

The exact form layout, status copy, endpoint envelope, and test prompt may be
chosen during implementation as long as secrets are never rendered in the
inspector, exported literally, logged, persisted, or used as metric labels.

## References

- LiteLLM provider connection flow:
  <https://docs.litellm.ai/docs/proxy/docker_quick_start>
- LiteLLM virtual keys, intentionally separate from this feature:
  <https://docs.litellm.ai/docs/proxy/virtual_keys>
