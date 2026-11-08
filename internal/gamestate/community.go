package gamestate

import (
	"fmt"
	"strconv"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

type GalacticCommunity struct {
	Members, Proposed, Passed, Failed, Emissaries OptionalIntList
	Formed                                        OptionalString
	Last, Voting                                  OptionalIntRef
	SavedFields                                   []clausewitz.Pair
	Unsupported                                   bool
}

type Resolution struct {
	Type, PassDate                    OptionalString
	Country                           OptionalIntRef
	Supporters, Opponents             OptionalIntList
	Votes                             []ResolutionVote
	VotingRecorded, VotingUnsupported bool
}

type ResolutionVote struct {
	Key     string
	Country *int
	Value   clausewitz.Value
}

func (c *GalacticCommunity) UnmarshalClausewitz(value clausewitz.Value) error {
	*c = GalacticCommunity{}
	if !value.IsObject() {
		c.Unsupported = true
		return nil
	}
	c.Members = communityIntList(value, "members")
	c.Proposed = communityIntList(value, "proposed")
	c.Passed = communityIntList(value, "passed")
	c.Failed = communityIntList(value, "failed")
	c.Emissaries = communityIntList(value, "emissaries")
	c.Formed = savedFieldString(value, "community_formed")
	c.Last, c.Voting = savedPathInt(value, "last"), savedPathInt(value, "voting")
	for _, pair := range value.Object.Pairs {
		switch pair.Key {
		case "category_timers", "type_timers", "cooldown", "days", "could_end_senate_session", "election", "emergency_fund":
			c.SavedFields = append(c.SavedFields, pair)
		}
	}
	return nil
}

func (r *Resolution) UnmarshalClausewitz(value clausewitz.Value) error {
	if !value.IsObject() {
		return fmt.Errorf("unsupported saved resolution shape")
	}
	*r = Resolution{
		Type: savedFieldString(value, "type"), PassDate: savedFieldString(value, "pass_date"), Country: savedPathInt(value, "country"),
		Supporters: communityIntList(value, "supporters"), Opponents: communityIntList(value, "opponents"),
	}
	voting, recorded, unsupported := savedFieldPath(value, "voting")
	r.VotingRecorded = recorded
	r.VotingUnsupported = unsupported || (recorded && !voting.IsObject())
	if recorded && !r.VotingUnsupported {
		for _, pair := range voting.Object.Pairs {
			entry := ResolutionVote{Key: pair.Key, Value: pair.Value}
			if id, err := strconv.Atoi(pair.Key); err == nil {
				entry.Country = &id
			}
			r.Votes = append(r.Votes, entry)
		}
	}
	return nil
}

func communityIntList(value clausewitz.Value, path ...string) OptionalIntList {
	field, recorded, unsupported := savedFieldPath(value, path...)
	refs := OptionalIntList{Recorded: recorded, Unsupported: unsupported}
	if recorded && !unsupported {
		refs.UnmarshalClausewitz(field)
	}
	return refs
}
