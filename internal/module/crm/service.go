package crm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Service provides CRM operations.
type Service struct {
	repo      Repository
	entitySvc *entity.Service
	bus       event.Bus
}

// NewService creates a new CRM service.
func NewService(repo Repository, entitySvc *entity.Service, bus event.Bus) *Service {
	return &Service{repo: repo, entitySvc: entitySvc, bus: bus}
}

// Create creates a new customer or supplier entity with CRM components.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, input CreateInput, actorID *uuid.UUID) (*Counterparty, error) {
	if input.Kind != "customer" && input.Kind != "supplier" {
		return nil, errs.NewBadRequest("kind must be customer or supplier")
	}
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}

	components := make(map[string]json.RawMessage)

	if input.Contact != nil {
		data, _ := json.Marshal(input.Contact)
		components["contact"] = data
	}

	profile := input.Profile
	if profile == nil {
		profile = make(map[string]any)
	}
	if _, ok := profile["tags"]; !ok {
		profile["tags"] = []string{}
	}
	profData, _ := json.Marshal(profile)
	components["crm_profile"] = profData

	balance := input.Balance
	if balance == nil {
		balance = map[string]any{
			"balance":            0,
			"credit_limit":       0,
			"payment_terms_days": 0,
		}
	}
	balData, _ := json.Marshal(balance)
	components["crm_balance"] = balData

	e, err := s.entitySvc.CreateWithComponents(ctx, orgID, entity.CreateEntityInput{
		Kind: input.Kind,
		Name: input.Name,
	}, components, actorID)
	if err != nil {
		return nil, err
	}

	cp := s.entityToCounterparty(e)

	s.publishEvent(ctx, orgID, e.ID, "crm."+input.Kind+".created", map[string]any{
		"id":   e.ID,
		"name": input.Name,
		"kind": input.Kind,
	}, actorID)

	return cp, nil
}

// Get returns a counterparty by ID with all components.
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Counterparty, error) {
	e, err := s.entitySvc.Get(ctx, orgID, id, true)
	if err != nil {
		return nil, err
	}
	if e.Kind != "customer" && e.Kind != "supplier" {
		return nil, errs.NewNotFound("counterparty not found")
	}
	return s.entityToCounterparty(e), nil
}

// List returns counterparties matching the filter.
func (s *Service) List(ctx context.Context, orgID uuid.UUID, kind string, filter CounterpartyFilter) (*types.PageResponse[Counterparty], error) {
	filter.Page.Normalize()

	entityFilter := entity.ListFilter{
		Kind:   &kind,
		Search: filter.Search,
		Page:   filter.Page,
	}

	resp, err := s.entitySvc.List(ctx, orgID, entityFilter, true)
	if err != nil {
		return nil, err
	}

	var counterparties []Counterparty
	for _, e := range resp.Items {
		cp := s.entityToCounterparty(&e)
		if filter.Tag != nil || filter.Category != nil {
			if !s.matchesFilter(cp, filter) {
				continue
			}
		}
		counterparties = append(counterparties, *cp)
	}
	if counterparties == nil {
		counterparties = []Counterparty{}
	}

	return &types.PageResponse[Counterparty]{
		Items:  counterparties,
		Total:  resp.Total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

// Update modifies a counterparty.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, input UpdateInput, actorID *uuid.UUID) (*Counterparty, error) {
	e, err := s.entitySvc.Get(ctx, orgID, id, true)
	if err != nil {
		return nil, err
	}
	if e.Kind != "customer" && e.Kind != "supplier" {
		return nil, errs.NewNotFound("counterparty not found")
	}

	if input.Name != nil {
		_, err := s.entitySvc.Update(ctx, orgID, id, entity.UpdateEntityInput{
			Name: input.Name,
		}, actorID)
		if err != nil {
			return nil, err
		}
	}

	if input.Contact != nil {
		data, _ := json.Marshal(input.Contact)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, id, "contact", data, actorID); err != nil {
			return nil, err
		}
	}
	if input.Profile != nil {
		data, _ := json.Marshal(input.Profile)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, id, "crm_profile", data, actorID); err != nil {
			return nil, err
		}
	}
	if input.Balance != nil {
		data, _ := json.Marshal(input.Balance)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, id, "crm_balance", data, actorID); err != nil {
			return nil, err
		}
	}

	s.publishEvent(ctx, orgID, id, "crm."+e.Kind+".updated", map[string]any{
		"id": id,
	}, actorID)

	return s.Get(ctx, orgID, id)
}

// UpdateTags modifies the tags on a counterparty's crm_profile.
func (s *Service) UpdateTags(ctx context.Context, orgID, id uuid.UUID, input TagsInput, actorID *uuid.UUID) (*Counterparty, error) {
	e, err := s.entitySvc.Get(ctx, orgID, id, true)
	if err != nil {
		return nil, err
	}
	if e.Kind != "customer" && e.Kind != "supplier" {
		return nil, errs.NewNotFound("counterparty not found")
	}

	profile := s.getComponentData(e, "crm_profile")
	if profile == nil {
		profile = make(map[string]any)
	}

	var tags []string
	if existing, ok := profile["tags"].([]any); ok {
		for _, t := range existing {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	for _, add := range input.Add {
		found := false
		for _, t := range tags {
			if t == add {
				found = true
				break
			}
		}
		if !found {
			tags = append(tags, add)
		}
	}

	for _, rm := range input.Remove {
		for i, t := range tags {
			if t == rm {
				tags = append(tags[:i], tags[i+1:]...)
				break
			}
		}
	}

	profile["tags"] = tags
	data, _ := json.Marshal(profile)
	if _, err := s.entitySvc.SetComponent(ctx, orgID, id, "crm_profile", data, actorID); err != nil {
		return nil, err
	}

	return s.Get(ctx, orgID, id)
}

// GetOrders returns order history for a customer.
func (s *Service) GetOrders(ctx context.Context, orgID, customerID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error) {
	page.Normalize()
	return s.repo.GetOrdersByCustomer(ctx, orgID, customerID, page)
}

// GetTransactions returns transaction history for a counterparty.
func (s *Service) GetTransactions(ctx context.Context, orgID, counterpartyID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error) {
	page.Normalize()
	return s.repo.GetTransactionsByCounterparty(ctx, orgID, counterpartyID, page)
}

// GetDeliveries returns delivery history for a supplier.
func (s *Service) GetDeliveries(ctx context.Context, orgID, supplierID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error) {
	page.Normalize()
	return s.repo.GetDeliveriesBySupplier(ctx, orgID, supplierID, page)
}

func (s *Service) entityToCounterparty(e *types.Entity) *Counterparty {
	cp := &Counterparty{Entity: *e}
	for _, c := range e.Components {
		var data map[string]any
		if err := json.Unmarshal(c.Data, &data); err != nil {
			continue
		}
		switch c.Type {
		case "crm_profile":
			cp.Profile = data
		case "crm_balance":
			cp.Balance = data
		case "contact":
			cp.Contact = data
		}
	}
	return cp
}

func (s *Service) getComponentData(e *types.Entity, compType string) map[string]any {
	for _, c := range e.Components {
		if c.Type == compType {
			var data map[string]any
			if err := json.Unmarshal(c.Data, &data); err == nil {
				return data
			}
		}
	}
	return nil
}

func (s *Service) matchesFilter(cp *Counterparty, filter CounterpartyFilter) bool {
	if filter.Tag != nil && cp.Profile != nil {
		tags, _ := cp.Profile["tags"].([]any)
		found := false
		for _, t := range tags {
			if ts, ok := t.(string); ok && ts == *filter.Tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filter.Category != nil && cp.Profile != nil {
		cat, _ := cp.Profile["category"].(string)
		if cat != *filter.Category {
			return false
		}
	}
	return true
}

func (s *Service) publishEvent(ctx context.Context, orgID, entityID uuid.UUID, eventType string, data map[string]any, actorID *uuid.UUID) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &entityID,
		Type:           eventType,
		Data:           payload,
		Timestamp:      time.Now(),
	}
	if actorID != nil {
		ev.ActorID = actorID
	}
	s.bus.Publish(ctx, ev)
}
