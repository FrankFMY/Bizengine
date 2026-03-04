-- 001_core: Core tables (users, workspaces, entities, components, events, processes, refresh_tokens)

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- users
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    full_name     TEXT NOT NULL DEFAULT '',
    phone         TEXT,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(lower(email));

-- workspaces
CREATE TABLE IF NOT EXISTS workspaces (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       TEXT NOT NULL,
    slug       TEXT NOT NULL UNIQUE,
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    plan       TEXT NOT NULL DEFAULT 'free',
    settings   JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- workspace_members
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         TEXT NOT NULL DEFAULT 'viewer',
    permissions  JSONB NOT NULL DEFAULT '[]',
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);

-- entities
CREATE TABLE IF NOT EXISTS entities (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind         TEXT NOT NULL,
    name         TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active',
    parent_id    UUID REFERENCES entities(id) ON DELETE SET NULL,
    meta         JSONB NOT NULL DEFAULT '{}',
    sort_order   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_entities_ws_kind ON entities(workspace_id, kind) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_entities_ws_status ON entities(workspace_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_entities_parent ON entities(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_entities_name_trgm ON entities USING gin(name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_entities_meta ON entities USING gin(meta jsonb_path_ops);
ALTER TABLE entities ENABLE ROW LEVEL SECURITY;

-- components
CREATE TABLE IF NOT EXISTS components (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity_id    UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL,
    type         TEXT NOT NULL,
    data         JSONB NOT NULL DEFAULT '{}',
    version      BIGINT NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, type)
);
CREATE INDEX IF NOT EXISTS idx_components_entity ON components(entity_id);
CREATE INDEX IF NOT EXISTS idx_components_ws_type ON components(workspace_id, type);
CREATE INDEX IF NOT EXISTS idx_components_data ON components USING gin(data jsonb_path_ops);
ALTER TABLE components ENABLE ROW LEVEL SECURITY;

-- events (partitioned by timestamp)
CREATE TABLE IF NOT EXISTS events (
    id           UUID NOT NULL DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL,
    entity_id    UUID,
    type         TEXT NOT NULL,
    data         JSONB NOT NULL DEFAULT '{}',
    actor_id     UUID,
    timestamp    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version      BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

CREATE TABLE IF NOT EXISTS events_default PARTITION OF events DEFAULT;

CREATE INDEX IF NOT EXISTS idx_events_ws_ts ON events(workspace_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_entity ON events(entity_id, timestamp DESC) WHERE entity_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_events_type ON events(workspace_id, type, timestamp DESC);
ALTER TABLE events ENABLE ROW LEVEL SECURITY;

-- process_definitions
CREATE TABLE IF NOT EXISTS process_definitions (
    id           TEXT NOT NULL,
    workspace_id UUID,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    definition   JSONB NOT NULL,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    version      INT NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_process_definitions_pk
    ON process_definitions (id, COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000'::UUID));

-- process_instances
CREATE TABLE IF NOT EXISTS process_instances (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    definition_id  TEXT NOT NULL,
    entity_id      UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    current_state  TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    context        JSONB NOT NULL DEFAULT '{}',
    history        JSONB NOT NULL DEFAULT '[]',
    started_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_pi_entity ON process_instances(entity_id);
CREATE INDEX IF NOT EXISTS idx_pi_ws_active ON process_instances(workspace_id) WHERE status = 'active';
ALTER TABLE process_instances ENABLE ROW LEVEL SECURITY;

-- refresh_tokens
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_rt_user ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
