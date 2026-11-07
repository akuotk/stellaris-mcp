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
	Recorded      bool
	MonthlyTrades MonthlyMarketTradeList `clausewitz:"monthly_trades"`
	Unsupported   bool
}

func (v *MarketOrders) UnmarshalClausewitz(value clausewitz.Value) error {
	if uniqueSavedFields(value, "monthly_trades") != nil {
		*v = MarketOrders{Recorded: true, Unsupported: true}
		return nil
	}
	type fields MarketOrders
	var decoded fields
	err := clausewitz.DecodeValue(value, &decoded)
	*v = MarketOrders(decoded)
	v.Recorded = true
	if err != nil {
		v.MonthlyTrades = MonthlyMarketTradeList{}
		v.Unsupported = true
	}
	return nil
}

type MonthlyMarketTradeList struct {
	Recorded    bool
	Items       []MonthlyMarketTrade
	Unsupported bool
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
