-- Users table for storing basic user information
-- Contains authentication credentials and user metadata
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    login_id VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE users IS 'Users table for storing basic user information';
COMMENT ON COLUMN users.id IS 'Primary key for user identification';
COMMENT ON COLUMN users.login_id IS 'Unique login identifier';
COMMENT ON COLUMN users.password IS 'Hashed password';
COMMENT ON COLUMN users.email IS 'User email address (unique)';
COMMENT ON COLUMN users.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN users.updated_at IS 'Record last update timestamp';

-- User role types table
-- Defines available system roles
CREATE TABLE user_role_types (
    id SERIAL PRIMARY KEY,
    role VARCHAR(16) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE user_role_types IS 'User role types table - defines available system roles';
COMMENT ON COLUMN user_role_types.id IS 'Primary key for role identification';
COMMENT ON COLUMN user_role_types.role IS 'Role name (admin, user, etc.)';
COMMENT ON COLUMN user_role_types.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN user_role_types.updated_at IS 'Record last update timestamp';

-- User roles junction table
-- Manages many-to-many relationship between users and roles
CREATE TABLE user_roles (
    user_id INTEGER NOT NULL REFERENCES users(id),
    role_id INTEGER NOT NULL REFERENCES user_role_types(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id)
);

COMMENT ON TABLE user_roles IS 'User roles junction table - manages many-to-many relationship between users and roles';
COMMENT ON COLUMN user_roles.user_id IS 'User ID (composite primary key)';
COMMENT ON COLUMN user_roles.role_id IS 'Role ID (foreign key)';
COMMENT ON COLUMN user_roles.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN user_roles.updated_at IS 'Record last update timestamp';

-- User scope types table
-- Defines available permission scopes in the system
CREATE TABLE user_scope_types (
    id SERIAL PRIMARY KEY,
    scope VARCHAR(64) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE user_scope_types IS 'User scope types table - defines available permission scopes in the system';
COMMENT ON COLUMN user_scope_types.id IS 'Primary key for scope identification';
COMMENT ON COLUMN user_scope_types.scope IS 'Scope name (read, write, etc.)';
COMMENT ON COLUMN user_scope_types.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN user_scope_types.updated_at IS 'Record last update timestamp';

-- User scopes junction table
-- Manages many-to-many relationship between users and scopes
CREATE TABLE user_scopes (
    user_id INTEGER NOT NULL REFERENCES users(id),
    scope_id INTEGER NOT NULL REFERENCES user_scope_types(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, scope_id)
);

COMMENT ON TABLE user_scopes IS 'User scopes junction table - manages many-to-many relationship between users and scopes';
COMMENT ON COLUMN user_scopes.user_id IS 'User ID (composite primary key)';
COMMENT ON COLUMN user_scopes.scope_id IS 'Scope ID (foreign key)';
COMMENT ON COLUMN user_scopes.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN user_scopes.updated_at IS 'Record last update timestamp';