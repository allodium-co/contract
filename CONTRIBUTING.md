# Contributing to Allodium contract

This repository is the canonical **wire contract** between an Allodium control
plane and a data-plane agent. Both sides — a public data plane and a private
control plane — depend on it, so it changes under one rule above all others.

## The one rule: additive-only

The contract is **versioned independently and evolves additively.** A change is
acceptable only if an old agent and a new control plane (and vice versa) keep
working across the supported version skew.

- **Allowed:** new messages, new fields with safe defaults, new RPCs, new enum
  values (where the receiver already tolerates unknowns).
- **Not allowed without a major-version process:** removing or renaming a field,
  changing a field's type or number, repurposing an existing field, tightening
  the meaning of an existing value, or making a previously-optional field
  required.

If you think you need a breaking change, open an issue describing the migration
first — don't send the PR.

## Keep the boundary honest

The contract is the enforcement point for the
[trust boundary](https://github.com/allodium-co/dataplane/blob/main/docs/TRUST-BOUNDARY.md).
A field must never create a channel for query data, query results,
table/catalog metadata, catalog grants, or storage credentials to travel from
the data plane up to the control plane. Desired state is **compute shape only**.
Reviewers will reject any field that widens what may cross the boundary.

## Making a change

1. **Open an issue first** for anything beyond a typo — describe the field, the
   compatibility impact, and the boundary impact.
2. **Edit the `.proto` and/or the desired-state JSON schema**, then regenerate
   stubs with the checked-in toolchain (do not hand-edit generated code).
3. **Run the checks:** `go build ./...`, `go vet ./...`, `go test ./...`, and the
   compatibility check that guards against non-additive changes.
4. **Update the reference docs** if you added or clarified a field.
5. **Open a PR** referencing the issue. By submitting, you agree your
   contribution is licensed under Apache-2.0 (see `LICENSE`).

## Commit and PR hygiene

- One logical change per PR.
- Explain *why*, and state the compatibility reasoning explicitly ("old agents
  ignore this field; default preserves prior behaviour").
- CI must be green, including the additive-only compatibility guard.
