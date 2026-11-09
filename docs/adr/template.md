# ADR-NNNN: <Short decision title>

- **Status:** Proposed | Accepted | Superseded by ADR-NNNN | Deprecated
- **Date:** YYYY-MM-DD
- **Owners:** <team or maintainers>
- **Decision scope:** <packages, connectors, or runtime behavior>

## Context

Describe the production problem, constraints, measurable requirements, and
assumptions. Link incidents, benchmarks, compatibility requirements, and
related ADRs.

## Decision

State the chosen approach and its invariants. Include operational defaults,
configuration ownership, failure behavior, and migration requirements.

## Consequences

List positive and negative outcomes, new operational responsibilities, and
what future changes now require an ADR.

## Alternatives considered

| Alternative | Rejected because |
| --- | --- |
| <option> | <trade-off> |

## Validation and rollback

Define tests, metrics, rollout gates, and the safe rollback path.

## Example: bounded channel strategy

**Decision:** Bound queued deliveries with `PipelineConfig.Buffer` and count
active workers separately. When capacity is exhausted, block source intake
rather than drop events or allocate unbounded memory.

**Consequences:** Throughput is limited by the slowest required sink, but
memory remains predictable and source-native backpressure can operate. Changing
this behavior requires load tests, delivery-semantics review, and a new ADR.
