package gamestate

import (
	"fmt"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type CountryEconomyModules struct {
	Recorded    bool
	Economy     *CountryEconomyModule `clausewitz:"standard_economy_module"`
	Unsupported bool
}

type CountryEconomyModule struct {
	Resources SavedResourceAmounts `clausewitz:"resources"`
}

func (v *CountryEconomyModule) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "resources"); err != nil {
		return err
	}
	type fields CountryEconomyModule
	return clausewitz.DecodeValue(value, (*fields)(v))
}

func (v *CountryEconomyModules) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "standard_economy_module") != nil {
		*v = CountryEconomyModules{Recorded: true, Unsupported: true}
		return nil
	}
	type fields CountryEconomyModules
	var decoded fields
	err := clausewitz.DecodeValue(value, &decoded)
	*v = CountryEconomyModules(decoded)
	v.Recorded = true
	if err != nil {
		v.Economy = nil
		v.Unsupported = true
	}
	return nil
}

type SavedResourceAmounts struct {
	Recorded    bool
	Entries     []SavedResourceAmount
	Unsupported bool
}

type SavedResourceAmount struct {
	Resource string
	Amount   string
}

func (v *SavedResourceAmounts) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = SavedResourceAmounts{Recorded: true}
	if !value.IsObject() {
		v.Unsupported = true
		return nil
	}
	for _, pair := range value.Object.Pairs {
		if !pair.Value.IsScalar() {
			v.Unsupported = true
			continue
		}
		v.Entries = append(v.Entries, SavedResourceAmount{Resource: pair.Key, Amount: pair.Value.Scalar})
	}
	return nil
}

type MarketOrders struct {
	Recorded             bool
	MonthlyTrades        MonthlyMarketTradeList     `clausewitz:"monthly_trades"`
	ResourcesBought      MarketResourceFlow         `clausewitz:"resources_bought"`
	ResourcesSold        MarketResourceFlow         `clausewitz:"resources_sold"`
	InternalFluctuations MarketInternalFluctuations `clausewitz:"internal_market_fluctuations"`
	GlobalDetails        []clausewitz.Pair          `clausewitz:"-"`
	Duplicate            bool                       `clausewitz:"-"`
	Unsupported          bool
}

func (v *MarketOrders) UnmarshalClausewitz(value clausewitz.Value) error {
	type fields MarketOrders
	var decoded fields
	err := clausewitz.DecodeValue(value, &decoded)
	*v = MarketOrders(decoded)
	v.Recorded = true
	if err != nil {
		*v = MarketOrders{Recorded: true, Unsupported: true}
		return nil
	}
	if uniqueSavedFields(value, "monthly_trades") != nil {
		v.MonthlyTrades = MonthlyMarketTradeList{Recorded: true, Duplicate: true, Unsupported: true}
	}
	if uniqueSavedFields(value, "resources_bought") != nil {
		v.ResourcesBought.MarketCountryRows = MarketCountryRows{Recorded: true, Duplicate: true}
	}
	if uniqueSavedFields(value, "resources_sold") != nil {
		v.ResourcesSold.MarketCountryRows = MarketCountryRows{Recorded: true, Duplicate: true}
	}
	if uniqueSavedFields(value, "internal_market_fluctuations") != nil {
		v.InternalFluctuations.MarketCountryRows = MarketCountryRows{Recorded: true, Duplicate: true}
	}
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "country", "enabled", "galactic_market_access", "galactic_market_resources", "fluctuations":
			v.GlobalDetails = append(v.GlobalDetails, pair)
		}
	}
	return nil
}

type MonthlyMarketTradeList struct {
	Recorded    bool
	Items       []MonthlyMarketTrade
	Unsupported bool
	Duplicate   bool
}

type MonthlyMarketTrade struct {
	ID        *string                `clausewitz:"id"`
	Amount    *string                `clausewitz:"amount"`
	Price     *string                `clausewitz:"price"`
	TradeData MonthlyMarketTradeData `clausewitz:"trade_data"`
}

type MonthlyMarketTradeData struct {
	Country   *int    `clausewitz:"country"`
	Resource  *string `clausewitz:"resource"`
	TradeType *string `clausewitz:"trade_type"`
}

func (v *MonthlyMarketTrade) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "id", "amount", "price", "trade_data"); err != nil {
		return err
	}
	type fields MonthlyMarketTrade
	return clausewitz.DecodeValue(value, (*fields)(v))
}

func (v *MonthlyMarketTradeData) UnmarshalClausewitz(value clausewitz.Value) error {
	if err := uniqueSavedFields(value, "country", "resource", "trade_type"); err != nil {
		return err
	}
	type fields MonthlyMarketTradeData
	return clausewitz.DecodeValue(value, (*fields)(v))
}

func (v *MonthlyMarketTradeList) UnmarshalClausewitz(value clausewitz.Value) error {
	*v = MonthlyMarketTradeList{Recorded: true}
	if value.IsObject() && len(value.Object.Pairs) > 0 {
		v.Unsupported = true
		return nil
	}
	if err := clausewitz.DecodeValue(value, &v.Items); err != nil {
		v.Items = nil
		v.Unsupported = true
	}
	return nil
}

type MarketCountryRows struct {
	Recorded       bool
	Duplicate      bool
	Unsupported    bool
	OrphanPayloads int
	Items          []MarketCountryRow
}

type MarketCountryRow struct {
	Country            OptionalIntRef
	Payload            *clausewitz.Value
	UnsupportedPayload bool
}

type MarketResourceFlow struct{ MarketCountryRows }
type MarketInternalFluctuations struct{ MarketCountryRows }

func (r *MarketResourceFlow) UnmarshalClausewitz(value clausewitz.Value) error {
	r.MarketCountryRows = decodeMarketCountryRows(value, "amount")
	return nil
}

func (r *MarketInternalFluctuations) UnmarshalClausewitz(value clausewitz.Value) error {
	r.MarketCountryRows = decodeMarketCountryRows(value, "resources")
	return nil
}

func decodeMarketCountryRows(value clausewitz.Value, payloadKey string) MarketCountryRows {
	rows := MarketCountryRows{Recorded: true}
	if !value.IsObject() {
		rows.Unsupported = true
		return rows
	}
	pairs := value.Object.Pairs
	for i := 0; i < len(pairs); i++ {
		pair := pairs[i]
		switch pair.Key {
		case "country":
			var row MarketCountryRow
			row.Country.UnmarshalClausewitz(pair.Value)
			if i+1 < len(pairs) && pairs[i+1].Key == payloadKey {
				i++
				row.Payload = &pairs[i].Value
				if payloadKey == "amount" {
					row.UnsupportedPayload = !row.Payload.IsList() && (!row.Payload.IsObject() || len(row.Payload.Object.Pairs) != 0)
					for _, item := range row.Payload.List {
						row.UnsupportedPayload = row.UnsupportedPayload || !item.IsScalar()
					}
				} else {
					row.UnsupportedPayload = !row.Payload.IsObject()
					if row.Payload.IsObject() {
						for _, item := range row.Payload.Object.Pairs {
							row.UnsupportedPayload = row.UnsupportedPayload || !item.Value.IsScalar()
						}
					}
				}
			}
			rows.Items = append(rows.Items, row)
		case payloadKey:
			rows.OrphanPayloads++
		default:
			rows.Unsupported = true
		}
	}
	return rows
}

func uniqueSavedFields(value clausewitz.Value, keys ...string) error {
	if !value.IsObject() {
		return nil
	}
	for _, key := range keys {
		found := false
		for _, pair := range value.Object.Pairs {
			if pair.Key != key {
				continue
			}
			if found {
				return fmt.Errorf("duplicate resource/market field %q", key)
			}
			found = true
		}
	}
	return nil
}
