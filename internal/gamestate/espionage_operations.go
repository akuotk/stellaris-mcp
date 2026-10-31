package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type EspionageOperations struct {
	Items        map[int]EspionageOperation
	DecodeErrors map[int]string
	TableIssue   bool
}

type EspionageOperation struct {
	SpyNetwork *int     `clausewitz:"spy_network"`
	Type       string   `clausewitz:"type"`
	Index      *int     `clausewitz:"index"`
	LastRoll   *float64 `clausewitz:"last_roll"`
	DaysLeft   *float64 `clausewitz:"days_left"`
	Difficulty *float64 `clausewitz:"difficulty"`
	Info       *float64 `clausewitz:"info"`
	Target     struct {
		Type     string `clausewitz:"type"`
		ID       *int   `clausewitz:"id"`
		OpenerID *int   `clausewitz:"opener_id"`
	} `clausewitz:"target"`
	Log   []EspionageOperationLog `clausewitz:"log"`
	Flags EspionageOperationFlags `clausewitz:"flags"`
}

type EspionageOperationLog struct {
	Title      string   `clausewitz:"title"`
	Tooltip    string   `clausewitz:"tooltip"`
	Roll       *float64 `clausewitz:"roll"`
	Skill      *float64 `clausewitz:"skill"`
	Info       *float64 `clausewitz:"info"`
	Difficulty *float64 `clausewitz:"difficulty"`
	Date       string   `clausewitz:"date"`
}

func (o *EspionageOperations) UnmarshalClausewitz(value clausewitz.Value) error {
	*o = EspionageOperations{Items: map[int]EspionageOperation{}, DecodeErrors: map[int]string{}}
	if !value.IsObject() {
		o.TableIssue = true
		return nil
	}
	found := false
	for _, wrapper := range value.Object.Pairs {
		if wrapper.Key != "operations" {
			continue
		}
		if found {
			o.TableIssue = true
		}
		found = true
		if !wrapper.Value.IsObject() {
			o.TableIssue = true
			continue
		}
		for _, pair := range wrapper.Value.Object.Pairs {
			id, err := strconv.Atoi(pair.Key)
			if err != nil {
				o.TableIssue = true
				continue
			}
			if pair.Value.IsScalar() && pair.Value.Scalar == "none" {
				continue
			}
			var operation EspionageOperation
			err = clausewitz.DecodeValue(pair.Value, &operation)
			if _, duplicate := o.Items[id]; duplicate {
				err = fmt.Errorf("duplicate operation ID")
			}
			o.Items[id] = operation
			if err != nil {
				o.DecodeErrors[id] = err.Error()
			}
		}
	}
	if !found && len(value.Object.Pairs) > 0 {
		o.TableIssue = true
	}
	return nil
}

// Saved operation flags may be scalar or date/duration records.
type EspionageOperationFlag struct {
	Scalar   *string `clausewitz:"-"`
	FlagDate *string `clausewitz:"flag_date"`
	FlagDays *string `clausewitz:"flag_days"`
}

func (f *EspionageOperationFlag) UnmarshalClausewitz(value clausewitz.Value) error {
	if value.IsScalar() {
		text := value.Scalar
		*f = EspionageOperationFlag{Scalar: &text}
		return nil
	}
	if !value.IsObject() {
		return fmt.Errorf("unsupported operation flag shape")
	}
	seen := map[string]bool{}
	for _, pair := range value.Object.Pairs {
		if pair.Key != "flag_date" && pair.Key != "flag_days" {
			return fmt.Errorf("unsupported operation flag field")
		}
		if seen[pair.Key] {
			return fmt.Errorf("duplicate operation flag field")
		}
		seen[pair.Key] = true
		if !pair.Value.IsScalar() {
			return fmt.Errorf("unsupported operation flag field value")
		}
	}
	type fields EspionageOperationFlag
	var decoded fields
	if err := clausewitz.DecodeValue(value, &decoded); err != nil {
		return err
	}
	*f = EspionageOperationFlag(decoded)
	return nil
}

type EspionageOperationFlags map[string]EspionageOperationFlag

func (f *EspionageOperationFlags) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported operation flags container")
	}
	*f = EspionageOperationFlags{}
	for _, pair := range value.Object.Pairs {
		if pair.Key == "" {
			return fmt.Errorf("empty operation flag name")
		}
		if _, duplicate := (*f)[pair.Key]; duplicate {
			return fmt.Errorf("duplicate operation flag name")
		}
		var flag EspionageOperationFlag
		if err := clausewitz.DecodeValue(pair.Value, &flag); err != nil {
			return err
		}
		(*f)[pair.Key] = flag
	}
	return nil
}
