package tools

import (
	"strconv"
	"testing"
)

func TestRequireStringArgsRejectsOnlyProvidedNonStrings(t *testing.T) {
	req := rawRequest(map[string]any{"save_id": "x", "kind": 3, "page_context": nil})
	if err := requireStringArgs(req, "save_id", "absent"); err != nil {
		t.Fatalf("string and absent arguments rejected: %v", err)
	}
	if err := requireStringArgs(req, "save_id", "kind"); err == nil || err.Error() != "kind must be a string" {
		t.Fatalf("number accepted or wrong message: %v", err)
	}
	if err := requireStringArgs(req, "page_context"); err == nil || err.Error() != "page_context must be a string" {
		t.Fatalf("explicit null accepted or wrong message: %v", err)
	}
}

func TestRecordIDBoundIsLargestExactIDForThePlatform(t *testing.T) {
	want := int64(1<<53 - 1) // largest integer a JSON number carries exactly
	if strconv.IntSize == 32 {
		want = 1<<31 - 1
	}
	if bound := int64(maxSaveRecordID); bound != want {
		t.Fatalf("record ID bound = %d, want %d", bound, want)
	}
	if asInt := maxRecordID; int64(asInt) != want {
		t.Fatalf("int record ID bound = %d, want %d", asInt, want)
	}
	// The bound is what handlers enforce, not only a constant.
	req := rawRequest(map[string]any{"id": float64(want), "over": float64(want) + 1})
	if got, err := rawInteger(req, "id", 0, 0, maxRecordID); err != nil || int64(got) != want {
		t.Fatalf("largest ID rejected: %d, %v", got, err)
	}
	if want < 1<<53-1 {
		if _, err := rawInteger(req, "over", 0, 0, maxRecordID); err == nil {
			t.Fatal("ID above the platform bound accepted")
		}
	}
	if _, err := rawInteger(rawRequest(map[string]any{"id": float64(1 << 53)}), "id", 0, 0, maxRecordID); err == nil {
		t.Fatal("2^53 accepted")
	}
}
