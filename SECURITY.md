# Security Policy

## Scope

This repository holds the **wire contract** between an Allodium control plane
and a data-plane agent: the `agentpb` gRPC service definitions and the
desired-state JSON schema. It contains no running services and no secrets.

Security-relevant issues here are almost always **contract-shape** problems —
for example, a message field that could let one side smuggle data across the
[trust boundary](https://github.com/allodium-co/dataplane/blob/main/docs/TRUST-BOUNDARY.md),
a change that breaks backward/forward compatibility, or an under-specified
field that permits an unsafe interpretation on one side.

## Reporting a vulnerability

**Do not open a public GitHub issue for a security report.**

Use GitHub's private vulnerability reporting ("Report a vulnerability" under
the **Security** tab), or email **security@allodium.se**.

Please include:

- the message / field / schema element involved,
- the compatibility or boundary property you believe is violated,
- and, if possible, a minimal example of the offending payload.

## Our commitment

- We acknowledge reports within **3 business days**.
- We aim to confirm or dispute within **10 business days**.
- We will credit reporters who wish to be named once a fix or mitigation ships.

## Supported versions

The contract is **additive-only** and versioned independently. Compatibility
policy (supported skew between agent and control plane, deprecation windows) is
defined alongside the contract; security fixes are applied to the current
minor line and backported to supported prior lines per that policy.
