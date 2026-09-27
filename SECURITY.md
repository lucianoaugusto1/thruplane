# Security policy

## Supported versions

Thruplane has not published its first release. Until versioned releases exist,
security fixes target the current default branch only.

## Report a vulnerability

Do not open a public issue for a suspected vulnerability.

After the repository is published on GitHub, use its **Security** tab and
**Report a vulnerability** to submit a private advisory. Before public launch,
the repository owner must enable GitHub Private Vulnerability Reporting.

If private reporting is unavailable, contact the repository owner privately
through the account that publishes the project. Do not include secrets, live
provider credentials, or customer prompt data in the initial message.

Include:

- Affected commit or version
- Reproduction steps using synthetic credentials and data
- Expected and observed behavior
- Potential impact
- Any known mitigation

## Response targets

These are project targets, not contractual SLAs:

- Acknowledge a complete report within three business days.
- Provide an initial severity assessment within seven business days.
- Coordinate disclosure after a fix or mitigation is available.

Enterprise response commitments will be documented in customer agreements only
after an Enterprise offering becomes available.

## Security boundaries

Thruplane must never log prompts, provider API keys, authorization headers, or
full request bodies. Operators remain responsible for network access controls,
TLS termination, secret storage, provider policies, and retention settings in
their deployment environment.
