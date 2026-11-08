package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type RelationsManager struct {
	Relation    []Relation
	Recorded    bool
	Duplicate   bool
	Unsupported bool
}

func (m *RelationsManager) UnmarshalClausewitz(value clausewitz.Value) error {
	*m = RelationsManager{Recorded: true}
	if !value.IsObject() {
		m.Unsupported = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		if pair.Key != "relation" {
			continue
		}
		items := pair.Value.List
		if !pair.Value.IsList() {
			if pair.Value.IsObject() && len(pair.Value.Object.Pairs) == 0 {
				continue
			}
			items = []clausewitz.Value{pair.Value}
		}
		for _, item := range items {
			var row Relation
			row.UnmarshalClausewitz(item)
			m.Relation = append(m.Relation, row)
		}
	}
	return nil
}

type Relation struct {
	OwnerRef         OptionalIntRef    `clausewitz:"owner"`
	CountryRef       OptionalIntRef    `clausewitz:"country"`
	Contact          OptionalString    `clausewitz:"contact"`
	Trust            OptionalString    `clausewitz:"trust"`
	RelationCurrent  OptionalString    `clausewitz:"relation_current"`
	CountryID        *int              `clausewitz:"-"`
	ContactRaw       *string           `clausewitz:"-"`
	OwnerDuplicate   bool              `clausewitz:"-"`
	CountryDuplicate bool              `clausewitz:"-"`
	ContactDuplicate bool              `clausewitz:"-"`
	TrustDuplicate   bool              `clausewitz:"-"`
	CurrentDuplicate bool              `clausewitz:"-"`
	Unsupported      bool              `clausewitz:"-"`
	SavedDetails     []clausewitz.Pair `clausewitz:"-"`
}

func (r *Relation) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields Relation
	if err := clausewitz.DecodeValue(value, (*fields)(r)); err != nil {
		*r = Relation{Unsupported: true}
		return nil
	}
	r.OwnerDuplicate = uniqueSavedFields(value, "owner") != nil
	r.CountryDuplicate = uniqueSavedFields(value, "country") != nil
	r.ContactDuplicate = uniqueSavedFields(value, "contact") != nil
	r.TrustDuplicate = uniqueSavedFields(value, "trust") != nil
	r.CurrentDuplicate = uniqueSavedFields(value, "relation_current") != nil
	if r.OwnerDuplicate {
		r.OwnerRef = OptionalIntRef{Recorded: true}
	}
	if r.CountryDuplicate {
		r.CountryRef = OptionalIntRef{Recorded: true}
	}
	r.CountryID = r.CountryRef.Value
	if r.Contact.Recorded && !r.Contact.Unsupported && !r.ContactDuplicate {
		r.ContactRaw = &r.Contact.Value
	}
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "relation_last_month", "communications", "border_range", "borders", "closed_borders", "forced_open_borders",
			"embassy", "truce", "commercial_pact", "research_agreement", "defensive_pact", "guarantee", "non_aggression_pledge",
			"migration_access", "secret_fealty", "subject", "is_rival", "rival_date", "shared_rivals", "num_favors", "threat",
			"friendly", "neutral", "hostile", "acquainted_with", "envoy_opinion_positive", "envoy_opinion_negative",
			"foreign_envoys", "diplo_action_dates", "flags", "modifier", "pre_communications_name":
			r.SavedDetails = append(r.SavedDetails, pair)
		}
	}
	return nil
}
