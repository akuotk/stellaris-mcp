package gamestate

import "testing"

func TestOperationFlagMissingFieldsStayMissing(t *testing.T) {
	gs, err := Load([]byte(`espionage_operations={operations={0={flags={date_only={flag_date="2201.01.01"} days_only={flag_days=0} empty={}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(gs.EspionageOperations.DecodeErrors) > 0 {
		t.Fatal("partial known flag shape rejected")
	}
	flags := gs.EspionageOperations.Items[0].Flags
	if flags["date_only"].FlagDays != nil || flags["days_only"].FlagDate != nil || flags["days_only"].FlagDays == nil || *flags["days_only"].FlagDays != "0" || flags["empty"].FlagDays != nil || flags["empty"].FlagDate != nil {
		t.Fatal("missing fields became recorded values")
	}
}

func TestOperationFlagUnknownDuplicateAndAnonymousShapesAreNotSilent(t *testing.T) {
	for _, flags := range []string{
		`{example={unknown=1}}`,
		`{example={flag_date="2201.01.01" flag_date="2201.01.02"}}`,
		`{example={flag_days=0 flag_days=1}}`,
		`{example=0 example=1}`,
		`{example={anonymous_value}}`,
		`{anonymous_value}`,
		`{example={flag_days={unexpected=1}}}`,
		`{""=0}`,
	} {
		gs, err := Load([]byte(`date="2201.01.01" espionage_operations={operations={0={spy_network=0 flags=` + flags + `}}}`))
		if err != nil {
			t.Fatal("optional unsupported flag shape broke full save")
		}
		if gs.Date != "2201.01.01" || len(gs.EspionageOperations.DecodeErrors) != 1 {
			t.Fatalf("flag shape silently discarded: %s", flags)
		}
	}
}
