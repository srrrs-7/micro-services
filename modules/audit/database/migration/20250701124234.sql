-- Audit IDs table for tracking audit processes
-- Central table that manages audit identifiers
CREATE TABLE audit_ids (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE audit_ids IS 'Audit IDs table for tracking audit processes';
COMMENT ON COLUMN audit_ids.id IS 'Primary key for audit identification';
COMMENT ON COLUMN audit_ids.created_at IS 'Audit creation timestamp';

-- Audit approvers table
-- Manages users who can approve audit requests
CREATE TABLE audit_approvers (
    audit_id INTEGER NOT NULL REFERENCES audit_ids(id),
    user_id INTEGER NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (audit_id)
);

COMMENT ON TABLE audit_approvers IS 'Audit approvers table - manages users who can approve audit requests';
COMMENT ON COLUMN audit_approvers.audit_id IS 'Audit ID (foreign key)';
COMMENT ON COLUMN audit_approvers.user_id IS 'Approver user ID (unique)';
COMMENT ON COLUMN audit_approvers.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN audit_approvers.updated_at IS 'Record last update timestamp';

-- Audit applicants table
-- Manages users who submit audit requests
CREATE TABLE audit_applicants (
    audit_id INTEGER NOT NULL REFERENCES audit_ids(id),
    user_id INTEGER NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (audit_id)
);

COMMENT ON TABLE audit_applicants IS 'Audit applicants table - manages users who submit audit requests';
COMMENT ON COLUMN audit_applicants.audit_id IS 'Audit ID (foreign key)';
COMMENT ON COLUMN audit_applicants.user_id IS 'Applicant user ID (unique)';
COMMENT ON COLUMN audit_applicants.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN audit_applicants.updated_at IS 'Record last update timestamp';