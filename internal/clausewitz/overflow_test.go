package clausewitz

import (
	"strconv"
	"testing"
)

func TestDecodeRejectsIntegersThatDoNotFit(t *testing.T) {
	var small struct {
		A int8  `clausewitz:"a"`
		B uint8 `clausewitz:"b"`
	}
	for _, input := range []string{"a=128", "b=256", "a=-129"} {
		if err := Unmarshal([]byte(input), &small); err == nil {
			t.Fatalf("%s decoded without error: %+v", input, small)
		}
	}
	var keyed map[int8]string
	if err := Unmarshal([]byte("300=x"), &keyed); err == nil {
		t.Fatalf("overflowing map key accepted: %v", keyed)
	}

	// 4294967295 is common in saves; it must never wrap to -1 on 32-bit platforms.
	var id struct {
		ID int `clausewitz:"id"`
	}
	err := Unmarshal([]byte("id=4294967295"), &id)
	switch strconv.IntSize {
	case 64:
		if err != nil || int64(id.ID) != 4294967295 {
			t.Fatalf("64-bit decode: %v %d", err, id.ID)
		}
	default:
		if err == nil {
			t.Fatalf("32-bit decode wrapped 4294967295 to %d", id.ID)
		}
	}
	var wrapped struct {
		ID int `clausewitz:"id"`
	}
	if err := Unmarshal([]byte("id=18446744073709551615"), &wrapped); err == nil {
		t.Fatalf("value beyond int64 accepted: %d", wrapped.ID)
	}
}
