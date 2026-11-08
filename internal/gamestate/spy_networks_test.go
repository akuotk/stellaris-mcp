package gamestate

import (
	"os"
	"testing"
)

func TestDiplomaticRefShapesAndOptionalNetworkModifiers(t *testing.T) {
	gs, err := Load([]byte(`country={0={espionage_manager={spy_networks={0 0}} first_contact={contacts={}}}
	1={espionage_manager={spy_networks=0} first_contact={contacts={0} contacts={1}}}
	2={espionage_manager={} espionage_manager={spy_networks={2}} first_contact=unknown}}
	leaders={0={location={type=spy_network id=0 id=1}}}
	spy_networks={0={owner=0 leader=0 power=00.1000 timed_modifiers={items={{modifier=example days=-0.0} {days=0 days=1} unknown}}}
	1={owner=0 timed_modifiers={items=unknown}}
	2={owner=0 timed_modifiers={} timed_modifiers={}}
	3={owner=0} 3=none}
	first_contacts={contacts={0={owner=0 owner=1} 1={owner=0} 1=none 2={owner=0}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c0, c1, c2 := gs.Country[0], gs.Country[1], gs.Country[2]
	if len(c0.SpyNetworkRefs.Values) != 2 || c0.SpyNetworkRefs.Values[0] != 0 || !c0.FirstContactRefs.Recorded || len(c0.FirstContactRefs.Values) != 0 ||
		!c1.SpyNetworkRefs.Unsupported || !c1.FirstContactRefs.Duplicate || !c2.SpyNetworkRefs.Duplicate || !c2.FirstContactRefs.Unsupported {
		t.Fatal("country diplomatic reference shapes lost")
	}
	n := gs.SpyNetworks.Items[0]
	if !gs.Leaders[0].LocationAmbiguous || n.Power == nil || *n.Power != "00.1000" || len(n.TimedModifiers.Items) != 3 ||
		n.TimedModifiers.Items[0].Days == nil || *n.TimedModifiers.Items[0].Days != "-0.0" || !n.TimedModifiers.Items[1].Duplicate ||
		!n.TimedModifiers.Items[2].Unsupported || !gs.SpyNetworks.Items[1].TimedModifiers.Unsupported ||
		!gs.SpyNetworks.Items[2].TimedModifiers.Duplicate || len(gs.SpyNetworks.DecodeErrors) != 1 {
		t.Fatal("network field precision, shape, duplicates or optional isolation lost")
	}
	if len(gs.FirstContacts.Items) != 1 || len(gs.FirstContacts.DecodeErrors) != 2 {
		t.Fatal("duplicate first-contact IDs or owner fields remained authoritative")
	}
	for _, raw := range []string{
		`spy_networks={0={owner=0}} spy_networks={}`,
		`first_contacts={contacts={0={owner=0}}} first_contacts={}`,
		`first_contacts={contacts={0={owner=0}} contacts={}}`,
	} {
		gs, err := Load([]byte(`date=2201.01.01 ` + raw))
		if err != nil || gs.Date != "2201.01.01" || len(gs.SpyNetworks.Items) != 0 || len(gs.FirstContacts.Items) != 0 ||
			(!gs.SpyNetworks.TableIssue && !gs.FirstContacts.TableIssue) {
			t.Fatal("duplicate root or table became authoritative or blocked unrelated data")
		}
	}
}

func TestDiplomaticRefsLocalCatalogIntegration(t *testing.T) {
	dir := os.Getenv("STELLARIS_TEST_SAVE_DIR")
	if dir == "" {
		t.Skip("set STELLARIS_TEST_SAVE_DIR for read-only local diplomatic reference coverage")
	}
	files, err := ListSaveFiles(dir)
	if err != nil || len(files) == 0 {
		t.Fatal("local save catalog unavailable or empty")
	}
	var networks, networkRefs, networkOwnerMatches, networkLeaderMatches, modifierLists, modifiers int
	var contacts, contactRefs, contactOwnerMatches, unsupported int
	for _, file := range files {
		gs, err := LoadFromZip(file.Path)
		if err != nil {
			t.Fatal("whole-save decoding failed")
		}
		if gs.SpyNetworks.TableIssue || gs.FirstContacts.TableIssue {
			unsupported++
		}
		unsupported += len(gs.SpyNetworks.DecodeErrors) + len(gs.FirstContacts.DecodeErrors)
		contacts += len(gs.FirstContacts.Items)
		for countryID, c := range gs.Country {
			if c.SpyNetworkRefs.Unsupported || c.SpyNetworkRefs.Duplicate || c.FirstContactRefs.Unsupported || c.FirstContactRefs.Duplicate {
				unsupported++
			}
			for _, ref := range c.SpyNetworkRefs.Values {
				networkRefs++
				if n, ok := gs.SpyNetworks.Items[ref]; ok && n.Owner != nil && *n.Owner == countryID {
					networkOwnerMatches++
				}
			}
			for _, ref := range c.FirstContactRefs.Values {
				contactRefs++
				if contact, ok := gs.FirstContacts.Items[ref]; ok && contact.Owner != nil && *contact.Owner == countryID {
					contactOwnerMatches++
				}
			}
		}
		for id, n := range gs.SpyNetworks.Items {
			networks++
			if n.Leader != nil {
				if leader, ok := gs.Leaders[*n.Leader]; ok && !leader.LocationAmbiguous &&
					leader.Location.Type == "spy_network" && leader.Location.IDRaw != nil && *leader.Location.IDRaw == id {
					networkLeaderMatches++
				}
			}
			if n.TimedModifiers.Recorded {
				modifierLists++
			}
			if n.TimedModifiers.Duplicate || n.TimedModifiers.Unsupported {
				unsupported++
			}
			for _, modifier := range n.TimedModifiers.Items {
				modifiers++
				if modifier.Duplicate || modifier.Unsupported {
					unsupported++
				}
			}
		}
	}
	t.Logf("local diplomatic coverage: saves=%d networks=%d network_country_refs=%d reciprocal_network_refs=%d reciprocal_network_leaders=%d modifier_lists=%d modifiers=%d contacts=%d contact_country_refs=%d reciprocal_contact_refs=%d unsupported=%d", len(files), networks, networkRefs, networkOwnerMatches, networkLeaderMatches, modifierLists, modifiers, contacts, contactRefs, contactOwnerMatches, unsupported)
	if unsupported != 0 {
		t.Fatal("local diplomatic shapes need additional support")
	}
}
