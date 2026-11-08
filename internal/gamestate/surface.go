package gamestate

import "github.com/bmassemin/stellaris-mcp/internal/clausewitz"

type SurfaceRefList struct {
	OptionalIntList
	Duplicate bool
}

func (r *SurfaceRefList) rejectDuplicate(value clausewitz.Value, key string) {
	if uniqueSavedFields(value, key) != nil {
		*r = SurfaceRefList{OptionalIntList: OptionalIntList{Recorded: true}, Duplicate: true}
	}
}

type Building struct {
	Type     string         `clausewitz:"type"`
	Position OptionalString `clausewitz:"position"`
	Ruined   OptionalString `clausewitz:"ruined"`
}

func (b *Building) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "type", "position", "ruined"); err != nil {
		return err
	}
	type fields Building
	return clausewitz.DecodeValue(value, (*fields)(b))
}

type District struct {
	Type     string         `clausewitz:"type"`
	Level    int            `clausewitz:"level"`
	LevelRaw *string        `clausewitz:"level"`
	ZoneRefs SurfaceRefList `clausewitz:"zones"`
}

func (d *District) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "type", "level"); err != nil {
		return err
	}
	type fields District
	if err := clausewitz.DecodeValue(value, (*fields)(d)); err != nil {
		return err
	}
	d.ZoneRefs.rejectDuplicate(value, "zones")
	return nil
}

type Zone struct {
	Type         string         `clausewitz:"type"`
	BuildingRefs SurfaceRefList `clausewitz:"buildings"`
}

func (z *Zone) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "type"); err != nil {
		return err
	}
	type fields Zone
	if err := clausewitz.DecodeValue(value, (*fields)(z)); err != nil {
		return err
	}
	z.BuildingRefs.rejectDuplicate(value, "buildings")
	return nil
}

type Deposit struct {
	Type     string        `clausewitz:"type"`
	SwapType string        `clausewitz:"swap_type"`
	Holder   DepositHolder `clausewitz:"deposit_holder"`
}

func (d *Deposit) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "type", "swap_type"); err != nil {
		return err
	}
	type fields Deposit
	if err := clausewitz.DecodeValue(value, (*fields)(d)); err != nil {
		return err
	}
	if uniqueSavedFields(value, "deposit_holder") != nil {
		d.Holder = DepositHolder{Recorded: true, Duplicate: true}
	}
	return nil
}

type DepositHolder struct {
	Type        *string        `clausewitz:"type"`
	ID          OptionalIntRef `clausewitz:"id"`
	Recorded    bool           `clausewitz:"-"`
	Duplicate   bool           `clausewitz:"-"`
	Unsupported bool           `clausewitz:"-"`
}

func (h *DepositHolder) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "type", "id") != nil {
		*h = DepositHolder{Recorded: true, Duplicate: true}
		return nil
	}
	type fields DepositHolder
	if err := clausewitz.DecodeValue(value, (*fields)(h)); err != nil {
		*h = DepositHolder{Recorded: true, Unsupported: true}
	}
	h.Recorded = true
	return nil
}

type ExternalSurfaceBuildings struct {
	Items       []ExternalSurfaceBuilding
	Recorded    bool
	Duplicate   bool
	Unsupported bool
}

func (b *ExternalSurfaceBuildings) UnmarshalClausewitz(value clausewitz.Value) error {
	*b = ExternalSurfaceBuildings{Recorded: true}
	if !value.IsList() && (!value.IsObject() || len(value.Object.Pairs) != 0) {
		b.Unsupported = true
		return nil
	}
	return clausewitz.DecodeValue(value, &b.Items)
}

func (b *ExternalSurfaceBuildings) rejectDuplicate(value clausewitz.Value) {
	if uniqueSavedFields(value, "externally_owned_buildings") != nil {
		*b = ExternalSurfaceBuildings{Recorded: true, Duplicate: true}
	}
}

type ExternalSurfaceBuilding struct {
	Owner        OptionalString `clausewitz:"building_owner"`
	OwnerType    OptionalString `clausewitz:"owner_type"`
	BuildingRefs SurfaceRefList `clausewitz:"buildings"`
	Unsupported  bool           `clausewitz:"-"`
	Duplicate    bool           `clausewitz:"-"`
}

func (b *ExternalSurfaceBuilding) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "building_owner", "owner_type") != nil {
		*b = ExternalSurfaceBuilding{Duplicate: true}
		return nil
	}
	type fields ExternalSurfaceBuilding
	if err := clausewitz.DecodeValue(value, (*fields)(b)); err != nil {
		*b = ExternalSurfaceBuilding{Unsupported: true}
	}
	b.BuildingRefs.rejectDuplicate(value, "buildings")
	return nil
}
