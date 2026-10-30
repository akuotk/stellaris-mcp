package gamestate

import "testing"

func TestResolveNameWithNestedVariables(t *testing.T) {
	l := &Localizer{entries: map[string]string{"empire_adjective": "Example", "war_template": "$1$ - $2$ Conflict", "system": "Alpha"}}
	name := LocalizedName{Key: "war_template", Variables: []NameVar{
		{Key: "1", Value: LocalizedName{Key: "empire_adjective"}},
		{Key: "2", Value: LocalizedName{Key: "system"}},
	}}
	if got := l.ResolveName(name); got != "Example - Alpha Conflict" {
		t.Fatalf("got %q", got)
	}
	planet := LocalizedName{Key: "PLANET_NAME_FORMAT", Variables: []NameVar{{Key: "PARENT", Value: LocalizedName{Key: "system"}}, {Key: "NUMERAL", Value: LocalizedName{Key: "III"}}}}
	if got := l.ResolveName(planet); got != "Alpha III" {
		t.Fatalf("got %q", got)
	}
}
