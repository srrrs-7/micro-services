-- Replace the legacy users-related scaffolding (initial mis-copy from auth)
-- with the audit_events schema described in docs/system-design.md §4.3 / §5.
-- Phase 1.0: single non-partitioned table. Monthly RANGE partitioning by
-- occurred_at + retention Cron land in Phase 1.2 (§14).

DROP TABLE IF EXISTS user_scopes;
DROP TABLE IF EXISTS user_scope_types;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS user_role_types;
DROP TABLE IF EXISTS users;

-- 5W1H audit event store. Append-only by application contract; the DB role
-- separation that revokes UPDATE/DELETE (design §5.2) is configured outside
-- this migration so devcontainer Postgres stays writable for tests.
CREATE TABLE audit_events (
    id              BIGSERIAL    PRIMARY KEY,
    event_id        UUID         UNIQUE NOT NULL,
    occurred_at     TIMESTAMPTZ  NOT NULL,
    recorded_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    schema_version  SMALLINT     NOT NULL DEFAULT 1,

    actor_type       VARCHAR(16)  NOT NULL,
    actor_id         VARCHAR(128) NOT NULL,
    actor_ip         INET,
    actor_user_agent TEXT,

    action          VARCHAR(128) NOT NULL,
    resource_type   VARCHAR(64)  NOT NULL,
    resource_id     VARCHAR(128),

    service         VARCHAR(32)  NOT NULL,
    environment     VARCHAR(16)  NOT NULL,
    region          VARCHAR(32),
    host            VARCHAR(128),

    reason          TEXT,
    request_id      VARCHAR(64),
    trace_id        VARCHAR(32),
    span_id         VARCHAR(16),

    method          VARCHAR(16)  NOT NULL,
    outcome         VARCHAR(16)  NOT NULL,
    severity        VARCHAR(16)  NOT NULL DEFAULT 'info',
    source          VARCHAR(32)  NOT NULL,

    details         JSONB,

    -- Phase 1.5 hash chain. Columns ship now (NULL-permitted) so backfill is
    -- a pure UPDATE in §14 Phase 1.5 without a schema migration.
    prev_hash       CHAR(64),
    hash            CHAR(64),

    CONSTRAINT chk_actor_type CHECK (actor_type IN ('user','service','system','anonymous')),
    CONSTRAINT chk_outcome    CHECK (outcome IN ('success','failure','denied')),
    CONSTRAINT chk_method     CHECK (method IN ('HTTP','gRPC','CLI','JOB','SYSTEM')),
    CONSTRAINT chk_severity   CHECK (severity IN ('debug','info','notice','warning','error','critical'))
);

CREATE INDEX idx_audit_events_occurred_desc ON audit_events (occurred_at DESC);
CREATE INDEX idx_audit_events_actor         ON audit_events (actor_type, actor_id, occurred_at DESC);
CREATE INDEX idx_audit_events_resource      ON audit_events (resource_type, resource_id, occurred_at DESC);
CREATE INDEX idx_audit_events_action        ON audit_events (action, occurred_at DESC);
CREATE INDEX idx_audit_events_request       ON audit_events (request_id) WHERE request_id IS NOT NULL;

COMMENT ON TABLE audit_events IS '5W1H audit event store (append-only). See docs/system-design.md §5.';
COMMENT ON COLUMN audit_events.id              IS 'Internal monotonic id; orders rows for the Phase 1.5 hash chain.';
COMMENT ON COLUMN audit_events.event_id        IS 'External idempotency key (UUID v4 / v7).';
COMMENT ON COLUMN audit_events.occurred_at     IS 'When the event happened (client-supplied, server-skew validated).';
COMMENT ON COLUMN audit_events.recorded_at     IS 'When this row was persisted (server clock).';
COMMENT ON COLUMN audit_events.schema_version  IS 'Row schema version for forward-compatible reads.';
COMMENT ON COLUMN audit_events.actor_type      IS 'Who: user | service | system | anonymous.';
COMMENT ON COLUMN audit_events.actor_id        IS 'Who: user_id / client_id / "system" / "-" (anonymous).';
COMMENT ON COLUMN audit_events.actor_ip        IS 'Who: source IP if available.';
COMMENT ON COLUMN audit_events.actor_user_agent IS 'Who: User-Agent header (HTTP only).';
COMMENT ON COLUMN audit_events.action          IS 'What: <domain>.<resource>.<verb> (lower-snake.dot, past tense).';
COMMENT ON COLUMN audit_events.resource_type   IS 'What: target resource kind.';
COMMENT ON COLUMN audit_events.resource_id     IS 'What: target resource id (NULL for set-level operations).';
COMMENT ON COLUMN audit_events.service         IS 'Where: producer service (auth | audit | queue | ...).';
COMMENT ON COLUMN audit_events.environment     IS 'Where: dev | staging | prod.';
COMMENT ON COLUMN audit_events.region          IS 'Where: cloud region tag.';
COMMENT ON COLUMN audit_events.host            IS 'Where: hostname / pod name.';
COMMENT ON COLUMN audit_events.reason          IS 'Why: business context (free text, sanitized client-side).';
COMMENT ON COLUMN audit_events.request_id      IS 'Why: same-request correlation id.';
COMMENT ON COLUMN audit_events.trace_id        IS 'Why: W3C trace-context trace id (32 hex chars).';
COMMENT ON COLUMN audit_events.span_id         IS 'Why: W3C trace-context span id (16 hex chars).';
COMMENT ON COLUMN audit_events.method          IS 'How: HTTP | gRPC | CLI | JOB | SYSTEM.';
COMMENT ON COLUMN audit_events.outcome         IS 'How: success | failure | denied.';
COMMENT ON COLUMN audit_events.severity        IS 'How: debug | info | notice | warning | error | critical (RFC 5424).';
COMMENT ON COLUMN audit_events.source          IS 'How: producer component (api | worker | ...).';
COMMENT ON COLUMN audit_events.details         IS 'Structured extra information (PII blocklist enforced; 32 KB cap).';
COMMENT ON COLUMN audit_events.prev_hash       IS 'SHA-256 of the previous row (Phase 1.5 hash chain).';
COMMENT ON COLUMN audit_events.hash            IS 'SHA-256 of canonical_json(this row || prev_hash) (Phase 1.5 hash chain).';
