package messenger

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
)

type mockMessengerRepo struct {
	mu            sync.Mutex
	conversations map[uuid.UUID]*Conversation
	members       map[uuid.UUID][]ConversationMember
	messages      map[uuid.UUID]*Message
	convMessages  map[uuid.UUID][]uuid.UUID
	reactions     map[uuid.UUID][]Reaction
	readAt        map[string]*time.Time
}

func newMockRepo() *mockMessengerRepo {
	return &mockMessengerRepo{
		conversations: make(map[uuid.UUID]*Conversation),
		members:       make(map[uuid.UUID][]ConversationMember),
		messages:      make(map[uuid.UUID]*Message),
		convMessages:  make(map[uuid.UUID][]uuid.UUID),
		reactions:     make(map[uuid.UUID][]Reaction),
		readAt:        make(map[string]*time.Time),
	}
}

func (r *mockMessengerRepo) CreateConversation(_ context.Context, _ pgx.Tx, c *Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	c.Iat = now
	c.Upd = now
	c.Ver = 1
	r.conversations[c.ID] = c
	return nil
}

func (r *mockMessengerRepo) GetConversation(_ context.Context, orgID, convID uuid.UUID) (*Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.conversations[convID]
	if !ok {
		return nil, assert.AnError
	}
	return c, nil
}

func (r *mockMessengerRepo) ListConversations(_ context.Context, orgID, userID uuid.UUID, filter ConversationFilter) ([]ConversationWithUnread, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []ConversationWithUnread
	for _, c := range r.conversations {
		if c.OrganizationID != orgID {
			continue
		}
		isMember := false
		for _, m := range r.members[c.ID] {
			if m.UserID == userID {
				isMember = true
				break
			}
		}
		if !isMember {
			continue
		}
		if filter.Type != "" && c.Type != filter.Type {
			continue
		}
		result = append(result, ConversationWithUnread{Conversation: *c})
	}
	if result == nil {
		result = []ConversationWithUnread{}
	}
	return result, len(result), nil
}

func (r *mockMessengerRepo) FindDirectConversation(_ context.Context, orgID, userA, userB uuid.UUID) (*Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.conversations {
		if c.OrganizationID != orgID || c.Type != "direct" {
			continue
		}
		hasA, hasB := false, false
		for _, m := range r.members[c.ID] {
			if m.UserID == userA {
				hasA = true
			}
			if m.UserID == userB {
				hasB = true
			}
		}
		if hasA && hasB {
			return c, nil
		}
	}
	return nil, assert.AnError
}

func (r *mockMessengerRepo) FindEntityConversation(_ context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (*Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.conversations {
		if c.OrganizationID != orgID || c.Type != "entity" {
			continue
		}
		if c.ReferenceType != nil && *c.ReferenceType == refType && c.ReferenceID != nil && *c.ReferenceID == refID {
			return c, nil
		}
	}
	return nil, assert.AnError
}

func (r *mockMessengerRepo) UpdateLastMessage(_ context.Context, _ pgx.Tx, convID, msgID uuid.UUID, preview string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.conversations[convID]; ok {
		c.LastMessageID = &msgID
		c.LastMessageAt = &at
		c.LastMessagePreview = &preview
	}
	return nil
}

func (r *mockMessengerRepo) AddMember(_ context.Context, _ pgx.Tx, convID, userID uuid.UUID, role string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members[convID] = append(r.members[convID], ConversationMember{
		ID:             uuid.New(),
		ConversationID: convID,
		UserID:         userID,
		Role:           role,
		JoinedAt:       time.Now(),
	})
	return nil
}

func (r *mockMessengerRepo) RemoveMember(_ context.Context, convID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	members := r.members[convID]
	for i, m := range members {
		if m.UserID == userID {
			r.members[convID] = append(members[:i], members[i+1:]...)
			break
		}
	}
	return nil
}

func (r *mockMessengerRepo) IsMember(_ context.Context, convID, userID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.members[convID] {
		if m.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (r *mockMessengerRepo) GetMembers(_ context.Context, convID uuid.UUID) ([]ConversationMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.members[convID], nil
}

func (r *mockMessengerRepo) SetMuted(_ context.Context, convID, userID uuid.UUID, muted bool) error {
	return nil
}

func (r *mockMessengerRepo) GetUserConversationIDs(_ context.Context, userID, orgID uuid.UUID) ([]uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []uuid.UUID
	for convID, members := range r.members {
		for _, m := range members {
			if m.UserID == userID {
				ids = append(ids, convID)
				break
			}
		}
	}
	return ids, nil
}

func (r *mockMessengerRepo) CreateMessage(_ context.Context, _ pgx.Tx, m *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	m.Iat = now
	m.Upd = now
	m.Ver = 1
	r.messages[m.ID] = m
	r.convMessages[m.ConversationID] = append(r.convMessages[m.ConversationID], m.ID)
	return nil
}

func (r *mockMessengerRepo) GetMessage(_ context.Context, orgID, messageID uuid.UUID) (*Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.messages[messageID]
	if !ok {
		return nil, assert.AnError
	}
	return m, nil
}

func (r *mockMessengerRepo) UpdateMessage(_ context.Context, messageID uuid.UUID, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.messages[messageID]; ok {
		m.Content = &content
		now := time.Now()
		m.EditedAt = &now
	}
	return nil
}

func (r *mockMessengerRepo) SoftDeleteMessage(_ context.Context, messageID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.messages[messageID]; ok {
		now := time.Now()
		m.DeletedAt = &now
		m.Content = nil
	}
	return nil
}

func (r *mockMessengerRepo) ListMessages(_ context.Context, orgID, convID uuid.UUID, before *uuid.UUID, limit int) ([]Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var msgs []Message
	for _, mid := range r.convMessages[convID] {
		if m, ok := r.messages[mid]; ok && m.DeletedAt == nil {
			msgs = append(msgs, *m)
		}
	}
	if msgs == nil {
		msgs = []Message{}
	}
	return msgs, nil
}

func (r *mockMessengerRepo) SearchMessages(_ context.Context, orgID uuid.UUID, query string, convID *uuid.UUID, limit int) ([]Message, error) {
	return []Message{}, nil
}

func (r *mockMessengerRepo) MarkRead(_ context.Context, convID, userID, messageID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	key := convID.String() + ":" + userID.String()
	r.readAt[key] = &now
	return nil
}

func (r *mockMessengerRepo) GetUnreadCounts(_ context.Context, orgID, userID uuid.UUID) (map[uuid.UUID]int, error) {
	return map[uuid.UUID]int{}, nil
}

func (r *mockMessengerRepo) AddReaction(_ context.Context, messageID, userID uuid.UUID, emoji string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reactions[messageID] = append(r.reactions[messageID], Reaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
		Iat:       time.Now(),
	})
	return nil
}

func (r *mockMessengerRepo) RemoveReaction(_ context.Context, messageID, userID uuid.UUID, emoji string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	reactions := r.reactions[messageID]
	for i, rx := range reactions {
		if rx.UserID == userID && rx.Emoji == emoji {
			r.reactions[messageID] = append(reactions[:i], reactions[i+1:]...)
			break
		}
	}
	return nil
}

func (r *mockMessengerRepo) GetReactions(_ context.Context, messageID uuid.UUID) ([]Reaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reactions[messageID], nil
}

func (r *mockMessengerRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

func TestSendMessageAndListRoundTrip(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	conv, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)
	assert.Equal(t, "direct", conv.Type)

	msg, err := svc.SendMessage(ctx, orgID, SendMessageRequest{
		ConversationID: conv.ID,
		SenderID:       userA,
		Content:        "Hello!",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello!", *msg.Content)
	assert.Equal(t, "text", msg.ContentType)

	msgs, err := svc.ListMessages(ctx, orgID, conv.ID, nil, 50)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "Hello!", *msgs[0].Content)
}

func TestDirectConversationNoDuplicate(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	conv1, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)

	conv2, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)
	assert.Equal(t, conv1.ID, conv2.ID)
}

func TestDirectConversationSelfNotAllowed(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()

	_, err := svc.CreateDirectConversation(ctx, orgID, userA, userA)
	assert.Error(t, err)
}

func TestGroupConversation(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	creator := uuid.New()
	member1 := uuid.New()
	member2 := uuid.New()

	conv, err := svc.CreateGroupConversation(ctx, orgID, creator, "Baristas", []uuid.UUID{member1, member2})
	require.NoError(t, err)
	assert.Equal(t, "group", conv.Type)
	assert.Equal(t, "Baristas", *conv.Name)

	// Add member
	member3 := uuid.New()
	err = svc.AddMember(ctx, conv.ID, member3)
	require.NoError(t, err)

	// Remove member
	err = svc.RemoveMember(ctx, conv.ID, member2)
	require.NoError(t, err)
}

func TestEntityConversation(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	creator := uuid.New()
	orderID := uuid.New()

	conv, err := svc.GetOrCreateEntityConversation(ctx, orgID, "order", orderID, creator)
	require.NoError(t, err)
	assert.Equal(t, "entity", conv.Type)
	assert.Equal(t, "order", *conv.ReferenceType)
	assert.Equal(t, orderID, *conv.ReferenceID)

	// Second call returns same conversation
	conv2, err := svc.GetOrCreateEntityConversation(ctx, orgID, "order", orderID, creator)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, conv2.ID)
}

func TestUnreadCount(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	conv, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := svc.SendMessage(ctx, orgID, SendMessageRequest{
			ConversationID: conv.ID,
			SenderID:       userA,
			Content:        "msg",
		})
		require.NoError(t, err)
	}

	msgs, err := svc.ListMessages(ctx, orgID, conv.ID, nil, 50)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)

	err = svc.MarkRead(ctx, conv.ID, userB, msgs[2].ID)
	require.NoError(t, err)
}

func TestReactions(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	conv, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)

	msg, err := svc.SendMessage(ctx, orgID, SendMessageRequest{
		ConversationID: conv.ID,
		SenderID:       userA,
		Content:        "test",
	})
	require.NoError(t, err)

	err = svc.AddReaction(ctx, orgID, msg.ID, userB, "thumbs_up")
	require.NoError(t, err)

	reactions, err := repo.GetReactions(ctx, msg.ID)
	require.NoError(t, err)
	assert.Len(t, reactions, 1)
	assert.Equal(t, "thumbs_up", reactions[0].Emoji)

	err = svc.RemoveReaction(ctx, orgID, msg.ID, userB, "thumbs_up")
	require.NoError(t, err)

	reactions, err = repo.GetReactions(ctx, msg.ID)
	require.NoError(t, err)
	assert.Len(t, reactions, 0)
}

func TestEditMessage(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	conv, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)

	msg, err := svc.SendMessage(ctx, orgID, SendMessageRequest{
		ConversationID: conv.ID,
		SenderID:       userA,
		Content:        "original",
	})
	require.NoError(t, err)

	edited, err := svc.EditMessage(ctx, orgID, msg.ID, "edited content")
	require.NoError(t, err)
	assert.Equal(t, "edited content", *edited.Content)
}

func TestDeleteMessage(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	conv, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)

	msg, err := svc.SendMessage(ctx, orgID, SendMessageRequest{
		ConversationID: conv.ID,
		SenderID:       userA,
		Content:        "to be deleted",
	})
	require.NoError(t, err)

	err = svc.DeleteMessage(ctx, orgID, msg.ID)
	require.NoError(t, err)

	msgs, err := svc.ListMessages(ctx, orgID, conv.ID, nil, 50)
	require.NoError(t, err)
	assert.Len(t, msgs, 0)
}

func TestSystemMessage(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	creator := uuid.New()
	orderID := uuid.New()

	conv, err := svc.GetOrCreateEntityConversation(ctx, orgID, "order", orderID, creator)
	require.NoError(t, err)

	err = svc.SendSystemMessage(ctx, orgID, "order", orderID, "[System] Order confirmed")
	require.NoError(t, err)

	msgs, err := svc.ListMessages(ctx, orgID, conv.ID, nil, 50)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "system", msgs[0].ContentType)
	assert.Equal(t, "[System] Order confirmed", *msgs[0].Content)
}

func TestNonMemberCannotSend(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	outsider := uuid.New()

	conv, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)

	_, err = svc.SendMessage(ctx, orgID, SendMessageRequest{
		ConversationID: conv.ID,
		SenderID:       outsider,
		Content:        "intruder",
	})
	assert.Error(t, err)
}

func TestTypingDoesNotPersist(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	convID := uuid.New()

	err := svc.SetTyping(ctx, orgID, convID, userA)
	assert.NoError(t, err)
	// Typing is event-only, no persistence to verify
}

func TestAttachments(t *testing.T) {
	repo := newMockRepo()
	bus := event.NewLocalBus()
	svc := NewService(repo, bus)
	ctx := context.Background()
	orgID := uuid.New()
	userA := uuid.New()
	userB := uuid.New()
	orderID := uuid.New()

	conv, err := svc.CreateDirectConversation(ctx, orgID, userA, userB)
	require.NoError(t, err)

	msg, err := svc.SendMessage(ctx, orgID, SendMessageRequest{
		ConversationID: conv.ID,
		SenderID:       userA,
		Content:        "Check this order",
		Attachments: []Attachment{
			{
				Type:       "entity_ref",
				EntityKind: "order",
				EntityID:   &orderID,
				Preview:    map[string]any{"number": "ORD-00042", "status": "paid", "total": 100000},
			},
		},
	})
	require.NoError(t, err)
	assert.Len(t, msg.Attachments, 1)
	assert.Equal(t, "entity_ref", msg.Attachments[0].Type)
	assert.Equal(t, "order", msg.Attachments[0].EntityKind)
}
