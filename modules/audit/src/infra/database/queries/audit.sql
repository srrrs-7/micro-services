-- name: InsertEvent :one
-- Idempotent insert. Returns the new row on first write; ON CONFLICT skips
-- the row but RETURNING then yields zero results, so callers detect the
-- duplicate by checking for sql.ErrNoRows and follow up with GetEventByEventID
-- to read the existing recorded_at (design-doc §7.2).
INSERT INTO audit_events (
    event_id,
    occurred_at,
    schema_version,
    actor_type,
    actor_id,
    actor_ip,
    actor_user_agent,
    action,
    resource_type,
    resource_id,
    service,
    environment,
    region,
    host,
    reason,
    request_id,
    trace_id,
    span_id,
    method,
    outcome,
    severity,
    source,
    details
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23
)
ON CONFLICT (event_id) DO NOTHING
RETURNING
    id,
    event_id,
    recorded_at
;

-- name: GetEventByEventID :one
SELECT
    *
FROM
    audit_events
WHERE
    event_id = $1
;

-- name: ListEventsByTimeRange :many
-- Phase 1.0 listing — keyset cursor pagination over (occurred_at DESC, id DESC).
-- Filters beyond from/to are applied in code for now; the indexes in
-- migrations/20260507075754_audit_events.sql cover actor/resource/action/request
-- so adding them here later is a query-only change.
SELECT
    *
FROM
    audit_events
WHERE
    occurred_at BETWEEN $1 AND $2
ORDER BY
    occurred_at DESC,
    id DESC
LIMIT $3
;
