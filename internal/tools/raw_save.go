package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const rawScalarPreviewBytes = 256

func registerRawSave(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("inspect_save_archive", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDestructiveHintAnnotation(false), mcp.WithString("save_id", mcp.Description("ID from list_save_games; omitted selects latest")), mcp.WithDescription("Inspect the selected local save ZIP: revision SHA-256 and paginated archive entry indexes/names/sizes. Use this revision for all subsequent byte/tree reads; server reads local ZIP and makes no network requests; MCP client receives requested data."), mcp.WithString("revision", mcp.Description("Previous archive revision, required for offset > 0; rejects a changed selected save")), mcp.WithNumber("offset", mcp.DefaultNumber(0)), mcp.WithNumber("limit", mcp.Description("Entries per page, 1..200"), mcp.DefaultNumber(50))), handleInspectSaveArchive)
	common := func(description string) []mcp.ToolOption {
		return []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(description),
			mcp.WithString("save_id", mcp.Description("ID from list_save_games; omitted selects latest")),
			mcp.WithString("revision", mcp.Required(), mcp.Description("SHA-256 returned by inspect_save_archive; stale revisions are rejected")),
			mcp.WithString("page_context", mcp.Description("Optional token from a previous page; detects changed operation, entry, path or search parameters. Without it, nonzero offsets are marked unverified.")),
			mcp.WithNumber("entry_index", mcp.Description("ZIP entry index returned by inspect_save_archive; -1 selects unique gamestate (default); -2 reads the exact ZIP container bytes (read_save_bytes only)"), mcp.DefaultNumber(-1)),
		}
	}
	options := common("Read exact uncompressed bytes of any entry in the latest local save ZIP, or exact ZIP container bytes with entry_index=-2. Base64 preserves comments, quoting, escapes, whitespace and unknown syntax. Repeat with next_offset until null; offsets and limits are bytes, not characters.")
	options = append(options, mcp.WithNumber("offset", mcp.DefaultNumber(0)), mcp.WithNumber("limit", mcp.Description("Byte count, 1..65536"), mcp.DefaultNumber(8192)))
	s.AddTool(mcp.NewTool("read_save_bytes", options...), handleReadSaveBytes)
	options = common("Navigate a semantic Clausewitz tree using ordered positional paths: empty path=root, /0=first pair, /0/1=its second child. Duplicate keys and anonymous list positions stay distinct. Immediate children are paginated, never depth-truncated. Scalar previews are limited and explicitly flagged; exact source bytes remain available via read_save_bytes. Parsing normalizes quoting/comments/escapes and supports mixed keyed and anonymous entries. Exact source byte ranges and quote/escape flags are retained; comments/trivia remain in source bytes.")
	options = append(options, mcp.WithString("path", mcp.DefaultString("")), mcp.WithNumber("offset", mcp.DefaultNumber(0)), mcp.WithNumber("limit", mcp.Description("Children per page, 1..200"), mcp.DefaultNumber(50)))
	s.AddTool(mcp.NewTool("read_save_tree", options...), handleReadSaveTree)
	options = common("Read a scalar value or object key's normalized semantic bytes at a positional tree path, in bounded base64 pages. Offsets/limits are bytes, including multibyte UTF-8 boundaries. Quoting/comments/escapes are normalized; use read_save_bytes for authoritative exact source bytes.")
	options = append(options, mcp.WithString("path", mcp.Required(), mcp.Description("Positional path returned by read_save_tree or search_save_tree")), mcp.WithString("part", mcp.Description("value (default) or key"), mcp.DefaultString("value")), mcp.WithNumber("offset", mcp.DefaultNumber(0)), mcp.WithNumber("limit", mcp.Description("Bytes per page, 1..65536"), mcp.DefaultNumber(8192)))
	s.AddTool(mcp.NewTool("read_save_scalar", options...), handleReadSaveScalar)
	options = common("Search semantic Clausewitz keys and scalar values below a positional path, case-insensitive literal substring. Results retain distinct duplicate-key/list paths and are paginated by match offset. Empty query matches every node. Explicit previews may be shortened; read_save_bytes provides exact bytes. Parser interpretation may be incomplete; exact bytes remain authoritative.")
	options = append(options, mcp.WithString("path", mcp.DefaultString("")), mcp.WithString("query", mcp.DefaultString("")), mcp.WithNumber("offset", mcp.DefaultNumber(0)), mcp.WithNumber("limit", mcp.Description("Matches per page, 1..200"), mcp.DefaultNumber(50)))
	s.AddTool(mcp.NewTool("search_save_tree", options...), handleSearchSaveTree)
}

type rawSaveArchive struct {
	path     string
	saveID   string
	revision string
	zip      *zip.Reader
	data     []byte
}

func openRawSave(dir string, selection ...string) (*rawSaveArchive, error) {
	id := ""
	if len(selection) > 0 {
		id = selection[0]
	}
	selected, err := gamestate.ResolveSaveByID(dir, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(selected.Path)
	if err != nil {
		return nil, fmt.Errorf("read selected save: %w", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("read selected save ZIP: %w", err)
	}
	sum := sha256.Sum256(data)
	return &rawSaveArchive{path: selected.Path, saveID: selected.ID, revision: hex.EncodeToString(sum[:]), zip: archive, data: data}, nil
}

func rawJSON(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return toolError(err), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleInspectSaveArchive(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := requireStringArgs(req, "save_id"); err != nil {
		return toolError(err), nil
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return toolError(err), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, len(archive.zip.File))
	if err != nil {
		return toolError(err), nil
	}
	if err := requireStringArgs(req, "revision"); err != nil {
		return toolError(err), nil
	}
	revision := req.GetString("revision", "")
	if offset > 0 && revision == "" {
		return toolError(fmt.Errorf("revision is required for archive pagination")), nil
	}
	if revision != "" && revision != archive.revision {
		return toolError(fmt.Errorf("selected save changed; restart inspect_save_archive pagination")), nil
	}
	limit, err := rawInteger(req, "limit", 50, 1, 200)
	if err != nil {
		return toolError(err), nil
	}
	end := offset + min(limit, len(archive.zip.File)-offset)
	entries := make([]map[string]any, 0, end-offset)
	for i := offset; i < end; i++ {
		f := archive.zip.File[i]
		entries = append(entries, map[string]any{"entry_index": i, "name": f.Name, "uncompressed_bytes": f.UncompressedSize64, "compressed_bytes": f.CompressedSize64, "is_directory": f.FileInfo().IsDir()})
	}
	var next *int
	if end < len(archive.zip.File) {
		next = &end
	}
	return rawJSON(map[string]any{"save_id": archive.saveID, "save_path": archive.path, "revision": archive.revision, "revision_algorithm": "sha256_archive_bytes", "entries": entries, "entry_count": len(archive.zip.File), "offset": offset, "next_offset": next})
}

func rawInteger(req mcp.CallToolRequest, key string, defaultValue, minValue, maxValue int) (int, error) {
	n, err := rawInteger64(req, key, int64(defaultValue), int64(minValue), int64(maxValue))
	return int(n), err
}

func rawInteger64(req mcp.CallToolRequest, key string, defaultValue, minValue, maxValue int64) (int64, error) {
	n := defaultValue
	invalid := func() error { return fmt.Errorf("%s must be an integer from %d to %d", key, minValue, maxValue) }
	if value, ok := req.GetArguments()[key]; ok {
		switch value := value.(type) {
		case float64:
			if math.Trunc(value) != value || value < -1<<63 || value >= 1<<63 {
				return 0, invalid()
			}
			n = int64(value)
		case int:
			n = int64(value)
		default:
			return 0, fmt.Errorf("%s must be a JSON number", key)
		}
	}
	if n < minValue || n > maxValue {
		return 0, invalid()
	}
	return n, nil
}

func openRawEntry(req mcp.CallToolRequest) (*rawSaveArchive, int, *zip.File, error) {
	if err := requireStringArgs(req, "revision", "path", "query", "part", "save_id", "page_context"); err != nil {
		return nil, 0, nil, err
	}
	revision := req.GetString("revision", "")
	if revision == "" {
		return nil, 0, nil, fmt.Errorf("revision is required; call inspect_save_archive first")
	}
	archive, err := openRawSave(saveDir, req.GetString("save_id", ""))
	if err != nil {
		return nil, 0, nil, err
	}
	if revision != archive.revision {
		return nil, 0, nil, fmt.Errorf("selected save changed; call inspect_save_archive again and restart pagination (current revision %s)", archive.revision)
	}
	index, err := rawInteger(req, "entry_index", -1, -2, len(archive.zip.File)-1)
	if err != nil {
		return nil, 0, nil, err
	}
	if index == -1 {
		for i, f := range archive.zip.File {
			if f.Name == "gamestate" {
				if index != -1 {
					return nil, 0, nil, fmt.Errorf("multiple gamestate entries; select entry_index explicitly")
				}
				index = i
			}
		}
		if index == -1 {
			return nil, 0, nil, fmt.Errorf("no gamestate entry; select entry_index from inspect_save_archive")
		}
	}
	if index == -2 {
		return archive, index, nil, nil
	}
	f := archive.zip.File[index]
	if f.FileInfo().IsDir() {
		return nil, 0, nil, fmt.Errorf("entry %d is a directory", index)
	}

	return archive, index, f, nil
}

// rawPageToken binds an optional continuation to the source and operation parameters.
func rawPageToken(archive *rawSaveArchive, index int, operation string, parameters ...string) string {
	parts := []string{archive.saveID, archive.revision, strconv.Itoa(index), operation}
	parts = append(parts, parameters...)
	encoded, _ := json.Marshal(parts)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
func validateRawPageContext[Offset int | int64](req mcp.CallToolRequest, offset Offset, token string) (bool, error) {
	provided := req.GetString("page_context", "")
	if provided != "" && provided != token {
		return false, fmt.Errorf("page_context does not match this source or operation; restart pagination")
	}
	return offset == 0 || provided != "", nil
}

const rawReadChunkSize = 32 * 1024

type rawChunkReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r rawChunkReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p[:min(len(p), rawReadChunkSize)])
	if canceled := r.ctx.Err(); canceled != nil {
		return n, canceled
	}
	return n, err
}

func handleReadSaveBytes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := ctx.Err(); err != nil {
		return toolError(err), nil
	}
	archive, index, f, err := openRawEntry(req)
	if err != nil {
		return toolError(err), nil
	}

	var stream io.ReadCloser
	var total int64
	var entryName *string
	source := "archive_entry"
	if index == -2 {
		total = int64(len(archive.data))
		stream = io.NopCloser(bytes.NewReader(archive.data))
		source = "archive_bytes"
	} else {
		if f.UncompressedSize64 > 1<<53-1 {
			return toolError(fmt.Errorf("entry size exceeds exact JSON integer range")), nil
		}
		total = int64(f.UncompressedSize64)
		entryName = &f.Name
	}
	offset, err := rawInteger64(req, "offset", 0, 0, total)
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 8192, 1, 65536)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, index, "read_save_bytes")
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	end := offset + min(int64(limit), total-offset)
	if index != -2 {
		stream, err = f.Open()
		if err != nil {
			return toolError(fmt.Errorf("open archive entry: %w", err)), nil
		}
	}
	defer stream.Close()
	reader := rawChunkReader{ctx, stream}
	// CopyN and ReadFull can discard an error returned together with the last requested bytes.
	skipped, err := io.Copy(io.Discard, io.LimitReader(reader, offset))
	if err == nil && skipped != offset {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		return toolError(fmt.Errorf("seek uncompressed entry: %w", err)), nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, end-offset))
	if err == nil && int64(len(data)) != end-offset {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		return toolError(fmt.Errorf("read archive bytes: %w", err)), nil
	}
	var crcVerified *bool
	if index != -2 {
		verified := false
		crcVerified = &verified
	}
	if end == total {
		var extra [1]byte
		n, err := reader.Read(extra[:])
		if err != io.EOF || n != 0 {
			if err == nil {
				err = fmt.Errorf("entry exceeds declared uncompressed size")
			}
			return toolError(fmt.Errorf("verify entry end/checksum: %w", err)), nil
		}
		if crcVerified != nil {
			*crcVerified = true
		}
	}
	if err := ctx.Err(); err != nil {
		return toolError(err), nil
	}
	var next *int64
	if end < total {
		next = &end
	}
	return rawJSON(map[string]any{"save_id": archive.saveID, "revision": archive.revision, "entry_index": index, "entry_name": entryName, "source": source, "encoding": "base64", "offset": offset, "returned_bytes": len(data), "total_bytes": total, "next_offset": next, "data": base64.StdEncoding.EncodeToString(data), "entry_crc_verified": crcVerified, "integrity_note": "ZIP CRC is verified only when a request reads through the entry end", "page_context": token, "pagination_context_verified": verified})
}

func rawChildCount(v *clausewitz.RawNode) int { return len(v.Children) }
func rawChild(v *clausewitz.RawNode, index int) (*clausewitz.RawNode, *string) {
	child := v.Children[index]
	if child.Key != nil {
		return child, &child.Key.Text
	}
	return child, nil
}

func rawAtPath(root *clausewitz.RawNode, path string) (*clausewitz.RawNode, *string, error) {
	v := root
	var key *string
	if path == "" {
		return v, nil, nil
	}
	if !strings.HasPrefix(path, "/") {
		return v, nil, fmt.Errorf("path must be empty or slash-separated nonnegative child indexes")
	}
	for _, component := range strings.Split(path[1:], "/") {
		index, err := strconv.Atoi(component)
		if err != nil || index < 0 || strconv.Itoa(index) != component {
			return v, nil, fmt.Errorf("invalid positional path component %q", component)
		}
		if index >= rawChildCount(v) {
			return v, nil, fmt.Errorf("path %q: child %d does not exist (children=%d)", path, index, rawChildCount(v))
		}
		v, key = rawChild(v, index)
	}
	return v, key, nil
}

func rawNode(path string, key *string, v *clausewitz.RawNode) map[string]any {
	node := map[string]any{"path": path, "children_count": rawChildCount(v)}
	if key != nil {
		preview := []byte(*key)
		node["key_bytes"] = len(preview)
		node["key_preview_truncated"] = len(preview) > rawScalarPreviewBytes
		if len(preview) > rawScalarPreviewBytes {
			preview = preview[:rawScalarPreviewBytes]
		}
		node["key"] = strings.ToValidUTF8(string(preview), "�")
		node["key_preview_base64"] = base64.StdEncoding.EncodeToString(preview)
	}
	node["kind"] = v.Kind
	node["entry_source_offset"], node["entry_source_bytes"] = v.EntryStart, v.End-v.EntryStart
	node["value_source_offset"], node["value_source_bytes"] = v.Start, v.End-v.Start
	if v.Key != nil {
		node["key_source_offset"], node["key_source_bytes"], node["key_quoted"], node["key_escaped"] = v.Key.Start, v.Key.End-v.Key.Start, v.Key.Quoted, v.Key.Escaped
	}
	if v.IsScalar() {
		node["scalar_quoted"], node["scalar_escaped"] = v.Scalar.Quoted, v.Scalar.Escaped
		preview := []byte(v.Scalar.Text)
		truncated := len(preview) > rawScalarPreviewBytes
		if truncated {
			preview = preview[:rawScalarPreviewBytes]
		}
		// Base64 preserves partial UTF-8 sequences at preview byte boundaries.
		node["scalar_preview_base64"] = base64.StdEncoding.EncodeToString(preview)
		node["scalar_preview"] = strings.ToValidUTF8(string(preview), "�")
		node["scalar_bytes"] = len(v.Scalar.Text)
		node["scalar_preview_truncated"] = truncated
	}
	return node
}

func rawTree(ctx context.Context, req mcp.CallToolRequest) (*rawSaveArchive, int, *clausewitz.RawNode, *string, error) {
	return rawTreeAtPath(ctx, req, req.GetString("path", ""))
}

func rawTreeAtPath(ctx context.Context, req mcp.CallToolRequest, path string) (*rawSaveArchive, int, *clausewitz.RawNode, *string, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, nil, nil, err
	}
	archive, index, f, err := openRawEntry(req)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	if index == -2 {
		return nil, 0, nil, nil, fmt.Errorf("ZIP container is binary; use read_save_bytes with entry_index=-2")
	}
	stream, err := f.Open()
	if err != nil {
		return nil, 0, nil, nil, fmt.Errorf("open archive entry: %w", err)
	}
	defer stream.Close()
	data, err := io.ReadAll(rawChunkReader{ctx, stream})
	if err != nil {
		return nil, 0, nil, nil, fmt.Errorf("read archive entry: %w", err)
	}
	obj, err := clausewitz.ParseRaw(ctx, data)
	if err != nil {
		return nil, 0, nil, nil, fmt.Errorf("semantic tree unsupported for this entry: %w; use read_save_bytes for exact complete content", err)
	}
	v, key, err := rawAtPath(obj, path)
	return archive, index, v, key, err
}

func rawTreePage(archive *rawSaveArchive, index int) map[string]any {
	return map[string]any{"save_id": archive.saveID, "revision": archive.revision, "entry_index": index, "entry_name": archive.zip.File[index].Name, "source_range_unit": "zero-based bytes in the uncompressed ZIP entry; lengths include original token quotes/braces", "representation": "ordered mixed Clausewitz AST; scalar/key text normalized, exact entry-byte ranges retained; comments/trivia available via read_save_bytes"}
}

func handleReadSaveTree(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	archive, index, v, key, err := rawTree(ctx, req)
	if err != nil {
		return toolError(err), nil
	}
	path := req.GetString("path", "")
	total := rawChildCount(v)
	offset, err := rawInteger(req, "offset", 0, 0, total)
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 50, 1, 200)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, index, "read_save_tree", path)
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	end := offset + min(limit, total-offset)
	children := make([]map[string]any, 0, end-offset)
	for i := offset; i < end; i++ {
		child, key := rawChild(v, i)
		node := rawNode(path+"/"+strconv.Itoa(i), key, child)
		node["child_index"] = i
		children = append(children, node)
	}
	var next *int
	if end < total {
		next = &end
	}
	result := rawTreePage(archive, index)
	result["node"], result["children"], result["offset"], result["next_offset"] = rawNode(path, key, v), children, offset, next
	result["page_context"], result["pagination_context_verified"] = token, verified
	return rawJSON(result)
}

func handleSearchSaveTree(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	archive, index, v, key, err := rawTree(ctx, req)
	if err != nil {
		return toolError(err), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, math.MaxInt32)
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 50, 1, 200)
	if err != nil {
		return toolError(err), nil
	}
	query := strings.ToLower(req.GetString("query", ""))
	token := rawPageToken(archive, index, "search_save_tree", req.GetString("path", ""), query)
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	matches, total, visited, err := indexedRawSearchPage(ctx, v, key, req.GetString("path", ""), query, offset, limit)
	if err != nil {
		return toolError(err), nil
	}
	if offset > total {
		return toolError(fmt.Errorf("offset %d exceeds total matches %d", offset, total)), nil
	}
	var next *int
	end := offset + len(matches)
	if end < total {
		next = &end
	}
	result := rawTreePage(archive, index)
	result["matches"], result["total_matches"], result["visited_nodes"], result["offset"], result["next_offset"] = matches, total, visited, offset, next
	result["page_context"], result["pagination_context_verified"] = token, verified
	return rawJSON(result)
}

func indexedRawSearchPage(ctx context.Context, root *clausewitz.RawNode, rootKey *string, prefix, query string, offset, limit int) ([]map[string]any, int, int, error) {
	type frame struct {
		node      *clausewitz.RawNode
		nextChild int
	}
	stack := []frame{{node: root, nextChild: -1}}
	var path []byte
	matches := make([]map[string]any, 0, limit)
	total, visited := 0, 0
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.nextChild == -1 {
			if err := ctx.Err(); err != nil {
				return nil, total, visited, err
			}
			visited++
			key := rootKey
			if len(stack) > 1 {
				key = nil
				if top.node.Key != nil {
					key = &top.node.Key.Text
				}
			}
			if query == "" || key != nil && strings.Contains(strings.ToLower(*key), query) || top.node.IsScalar() && strings.Contains(strings.ToLower(top.node.Scalar.Text), query) {
				if total >= offset && len(matches) < limit {
					path = append(path[:0], prefix...)
					// Each ancestor's last selected child leads to the current node.
					for _, parent := range stack[:len(stack)-1] {
						path = append(path, '/')
						path = strconv.AppendInt(path, int64(parent.nextChild-1), 10)
					}
					matches = append(matches, rawNode(string(path), key, top.node))
				}
				total++
			}
			top.nextChild = 0
		}
		if top.nextChild == len(top.node.Children) {
			stack = stack[:len(stack)-1]
			continue
		}
		child := top.node.Children[top.nextChild]
		top.nextChild++
		stack = append(stack, frame{node: child, nextChild: -1})
	}
	return matches, total, visited, nil
}

func handleReadSaveScalar(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	archive, index, v, key, err := rawTree(ctx, req)
	if err != nil {
		return toolError(err), nil
	}
	part := req.GetString("part", "value")
	var text string
	switch part {
	case "value":
		if !v.IsScalar() {
			return toolError(fmt.Errorf("path must select a scalar; selected node has %d children", rawChildCount(v))), nil
		}
		text = v.Scalar.Text
	case "key":
		if key == nil {
			return toolError(fmt.Errorf("selected node has no object key")), nil
		}
		text = *key
	default:
		return toolError(fmt.Errorf("part must be value or key")), nil
	}
	offset, err := rawInteger(req, "offset", 0, 0, len(text))
	if err != nil {
		return toolError(err), nil
	}
	limit, err := rawInteger(req, "limit", 8192, 1, 65536)
	if err != nil {
		return toolError(err), nil
	}
	token := rawPageToken(archive, index, "read_save_scalar", req.GetString("path", ""), part)
	verified, err := validateRawPageContext(req, offset, token)
	if err != nil {
		return toolError(err), nil
	}
	end := offset + min(limit, len(text)-offset)
	var next *int
	if end < len(text) {
		next = &end
	}
	result := rawTreePage(archive, index)
	result["path"], result["part"], result["encoding"], result["offset"], result["returned_bytes"], result["total_bytes"], result["next_offset"], result["data"] = req.GetString("path", ""), part, "base64", offset, end-offset, len(text), next, base64.StdEncoding.EncodeToString([]byte(text[offset:end]))
	result["page_context"], result["pagination_context_verified"] = token, verified
	return rawJSON(result)
}
