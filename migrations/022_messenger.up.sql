-- 022_messenger: Contextual communication layer

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    type VARCHAR(20) NOT NULL,
    name VARCHAR(200),
    reference_type VARCHAR(50),
    reference_id UUID,
    avatar_file_id UUID REFERENCES files(id),
    created_by UUID NOT NULL REFERENCES users(id),
    last_message_id UUID,
    last_message_at TIMESTAMPTZ,
    last_message_preview TEXT,
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_conv_org ON conversations(organization_id);
CREATE INDEX idx_conv_ref ON conversations(organization_id, reference_type, reference_id) WHERE reference_type IS NOT NULL;

CREATE TABLE conversation_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    muted BOOLEAN NOT NULL DEFAULT false,
    last_read_message_id UUID,
    last_read_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (conversation_id, user_id)
);

CREATE INDEX idx_conv_member_user ON conversation_members(user_id);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    sender_id UUID NOT NULL REFERENCES users(id),
    content TEXT,
    content_type VARCHAR(20) NOT NULL DEFAULT 'text',
    attachments JSONB NOT NULL DEFAULT '[]',
    reply_to_id UUID REFERENCES messages(id),
    edited_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    ver INTEGER NOT NULL DEFAULT 1,
    upd TIMESTAMPTZ NOT NULL DEFAULT now(),
    iat TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_msg_conv ON messages(conversation_id, iat DESC);
CREATE INDEX idx_msg_sender ON messages(sender_id);
CREATE INDEX idx_msg_org ON messages(organization_id);

CREATE TABLE message_reactions (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    emoji VARCHAR(10) NOT NULL,
    iat TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id, emoji)
);

-- auto_ver_upd triggers
DO $$
DECLARE
    tbl TEXT;
BEGIN
    FOR tbl IN
        SELECT unnest(ARRAY[
            'conversations', 'conversation_members', 'messages'
        ])
    LOOP
        EXECUTE format(
            'DROP TRIGGER IF EXISTS trg_ver_upd ON %I; CREATE TRIGGER trg_ver_upd BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION auto_ver_upd();',
            tbl, tbl
        );
    END LOOP;
END;
$$;
