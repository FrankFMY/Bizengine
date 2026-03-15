-- 022_messenger rollback
DROP TRIGGER IF EXISTS trg_ver_upd ON messages;
DROP TRIGGER IF EXISTS trg_ver_upd ON conversation_members;
DROP TRIGGER IF EXISTS trg_ver_upd ON conversations;
DROP TABLE IF EXISTS message_reactions;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversation_members;
DROP TABLE IF EXISTS conversations;
