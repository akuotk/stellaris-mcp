package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rawReadCancelContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *rawReadCancelContext) Err() error {
	c.checks--
	if c.checks == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestRawReadsCancelDuringZIPEntry(t *testing.T) {
	syntheticSaveDir(t, []syntheticEntry{{"gamestate", []byte("table={0={value=" + strings.Repeat("x", 3*rawReadChunkSize) + "}}}")}})
	revision := rawRevision(t)
	for _, tc := range []struct {
		name, want string
		handler    rawHandler
		args       map[string]any
		checks     int
	}{
		{"tree", "read archive entry", handleReadSaveTree, nil, 5},
		{"search", "read archive entry", handleSearchSaveTree, nil, 5},
		{"scalar", "read archive entry", handleReadSaveScalar, map[string]any{"path": "/0/0/0"}, 5},
		{"record refs", "read archive entry", handleSaveRecordRefs, map[string]any{"table_path": "/0", "record_id": 0}, 5},
		{"byte skip", "seek uncompressed entry", handleReadSaveBytes, map[string]any{"offset": 65536}, 5},
		{"byte read", "read archive bytes", handleReadSaveBytes, map[string]any{"limit": 65536}, 5},
		{"byte exact final requested chunk", "read archive bytes", handleReadSaveBytes, map[string]any{"limit": 512}, 3},
		{"byte exact skip chunk", "seek uncompressed entry", handleReadSaveBytes, map[string]any{"offset": 8192}, 3},
		{"container bytes", "read archive bytes", handleReadSaveBytes, map[string]any{"entry_index": -2}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{"revision": revision}
			for k, v := range tc.args {
				args[k] = v
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			counting := &rawReadCancelContext{Context: ctx, cancel: cancel, checks: tc.checks}
			result, err := tc.handler(counting, rawRequest(args))
			if err != nil || result == nil || !result.IsError || !strings.Contains(resultText(result), tc.want+": context canceled") {
				t.Fatalf("expected cancellation inside %s; result=%v error=%v", tc.want, result, err)
			}
			result, err = tc.handler(ctx, rawRequest(args))
			if err != nil || result == nil || !result.IsError || !strings.Contains(resultText(result), "context canceled") {
				t.Fatalf("expected precanceled request; result=%v error=%v", result, err)
			}
		})
	}
}

type rawChunkTestReader struct {
	t      *testing.T
	cancel context.CancelFunc
}

func (r rawChunkTestReader) Read(p []byte) (int, error) {
	r.t.Helper()
	if len(p) != rawReadChunkSize {
		r.t.Fatalf("underlying read size = %d, want %d", len(p), rawReadChunkSize)
	}
	r.cancel()
	return len(p), io.EOF
}

func TestRawChunkReaderBoundsAndChecksAfterRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := rawChunkReader{ctx, rawChunkTestReader{t, cancel}}
	if n, err := r.Read(make([]byte, 2*rawReadChunkSize)); n != rawReadChunkSize || err != context.Canceled {
		t.Fatalf("got (%d, %v) after canceled underlying read", n, err)
	}
	if n, err := r.Read(make([]byte, 1)); n != 0 || err != context.Canceled {
		t.Fatalf("got (%d, %v) for a read after cancellation", n, err)
	}
}

func rawSyntheticStoredZIP(t *testing.T, payload []byte) ([]byte, int) {
	t.Helper()
	var buffer bytes.Buffer
	w := zip.NewWriter(&buffer)
	entry, err := w.CreateHeader(&zip.FileHeader{Name: "gamestate", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	offset, err := archive.File[0].DataOffset()
	if err != nil {
		t.Fatal(err)
	}
	return data, int(offset)
}

func rawUseSyntheticZIP(t *testing.T, data []byte) string {
	t.Helper()
	dir := syntheticSaveDir(t, nil)
	if err := os.WriteFile(filepath.Join(dir, "synthetic.sav"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return rawRevision(t)
}

func TestRawReadsCorruptZIPPartialAndEnd(t *testing.T) {
	payload := []byte("table={0={value=synthetic}}")
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprintf("corrupt=%t", corrupt), func(t *testing.T) {
			data, start := rawSyntheticStoredZIP(t, payload)
			if corrupt {
				data[start+len(payload)-1] = '!'
			}
			revision := rawUseSyntheticZIP(t, data)
			page := rawCall(t, handleReadSaveBytes, map[string]any{"revision": revision, "limit": 1})
			if page["data"] != base64.StdEncoding.EncodeToString(payload[:1]) || page["entry_crc_verified"] != false {
				t.Fatalf("partial prefix or CRC status incorrect: %v", page)
			}
			final := map[string]any{"revision": revision, "offset": 1, "page_context": page["page_context"]}
			if !corrupt {
				end := rawCall(t, handleReadSaveBytes, final)
				if end["data"] != base64.StdEncoding.EncodeToString(payload[1:]) || end["entry_crc_verified"] != true {
					t.Fatalf("verified continuation changed bytes or CRC status: %v", end)
				}
				return
			}
			for _, tc := range []struct {
				handler rawHandler
				args    map[string]any
			}{
				{handleReadSaveBytes, final},
				{handleReadSaveBytes, map[string]any{"revision": revision, "offset": len(payload)}},
				{handleReadSaveTree, map[string]any{"revision": revision}},
			} {
				result, err := tc.handler(context.Background(), rawRequest(tc.args))
				if err != nil || result == nil || !result.IsError || !strings.Contains(resultText(result), "checksum error") {
					t.Fatalf("corrupt complete read exposed a success or wrong error: result=%v error=%v", result, err)
				}
			}
		})
	}
}

func TestRawByteOffsetsBeyond32Bit(t *testing.T) {
	data, _ := rawSyntheticStoredZIP(t, []byte("key=synthetic"))
	central := bytes.Index(data, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("missing synthetic central directory")
	}
	const declared = 1<<31 + 16
	binary.LittleEndian.PutUint32(data[central+24:], declared)
	revision := rawUseSyntheticZIP(t, data)
	page := rawCall(t, handleReadSaveBytes, map[string]any{"revision": revision, "limit": 1})
	if page["total_bytes"] != float64(declared) || page["next_offset"] != float64(1) || page["entry_crc_verified"] != false {
		t.Fatalf("large declared size was narrowed: %v", page)
	}
	for _, tc := range []struct {
		offset float64
		want   string
	}{
		{math.MaxInt32 + 1, "seek uncompressed entry: unexpected EOF"},
		{declared, "seek uncompressed entry: unexpected EOF"},
		{declared + 1, "offset must be an integer"},
		{1 << 53, "offset must be an integer"},
		{1 << 63, "offset must be an integer"},
	} {
		result, err := handleReadSaveBytes(context.Background(), rawRequest(map[string]any{"revision": revision, "offset": tc.offset}))
		if err != nil || result == nil || !result.IsError || !strings.Contains(resultText(result), tc.want) {
			t.Fatalf("offset %.0f: result=%v error=%v; expected %q", tc.offset, result, err, tc.want)
		}
	}
}
