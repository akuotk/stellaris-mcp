package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestGalaxyMetadataRetainsRootAndFieldOrder(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`galaxy={seed=0 design={type=first} design={type=second} mixed={alpha key=value} value=0.123456789} galaxy=none`)}})
	s := server.NewMCPServer("test", "test")
	registerGalaxyMetadata(s)
	if _, ok := s.ListTools()["get_galaxy_metadata"]; !ok {
		t.Fatal("galaxy metadata tool not registered")
	}
	result, err := handleGalaxyMetadata(context.Background(), rawRequest(nil))
	if err != nil || result.IsError {
		t.Fatal("galaxy metadata request failed")
	}
	got := resultText(result)
	for _, want := range []string{"Multiple root galaxy records", `key="seed": "0"`, `key="design" [duplicate saved key]`, "mixed saved block", `"0.123456789"`, `galaxy[1] (saved order): "none"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	if strings.Index(got, `"first"`) > strings.Index(got, `"second"`) {
		t.Fatal("duplicate field order changed")
	}
}

func TestGalaxyMetadataMissingAndInvalidSaveID(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte(`date=2200.01.01`)}})
	result, err := handleGalaxyMetadata(context.Background(), rawRequest(nil))
	if err != nil || result.IsError || !strings.Contains(resultText(result), "galaxy not recorded") {
		t.Fatal("missing galaxy not reported")
	}
	result, err = handleGalaxyMetadata(context.Background(), rawRequest(map[string]any{"save_id": 0}))
	if err != nil || !result.IsError {
		t.Fatal("invalid save_id accepted")
	}
}
