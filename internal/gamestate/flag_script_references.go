package gamestate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bmassemin/stellaris-mcp/internal/clausewitz"
)

const (
	maxFlagScriptBytes    = 1 << 20
	maxFlagScanBytes      = 128 << 20
	maxFlagWalkEntries    = 8192
	maxFlagArchives       = 32
	maxFlagArchiveBytes   = 128 << 20
	maxFlagZipDirectory   = 4 << 20
	maxFlagZipMembers     = 4096
	maxFlagScriptKeys     = 8192
	maxFlagScriptKeyBytes = 256
)

var flagScriptCommands = [...]string{"set_global_flag", "set_timed_global_flag", "has_global_flag", "remove_global_flag"}

type FlagScriptReference struct {
	Command      string
	Sites, Files int
}

type FlagScriptIndex struct {
	Status, Fingerprint string
	byKey               map[string][len(flagScriptCommands)]FlagScriptReference
}

func (index FlagScriptIndex) References(key string) [len(flagScriptCommands)]FlagScriptReference {
	refs := index.byKey[key]
	for i, command := range flagScriptCommands {
		refs[i].Command = command
	}
	return refs
}

// FlagScriptSource captures one immutable, save-independent snapshot per process registration.
type FlagScriptSource func() FlagScriptIndex

func NewFlagScriptSource(localizationDir string) FlagScriptSource {
	return sync.OnceValue(func() FlagScriptIndex {
		root := flagGameRoot(localizationDir)
		if root == nil {
			return FlagScriptIndex{Status: "unavailable", Fingerprint: "unavailable"}
		}
		defer root.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		scan := flagScriptScan{
			ctx: ctx, digest: sha256.New(),
			index: FlagScriptIndex{Status: "complete", byKey: make(map[string][len(flagScriptCommands)]FlagScriptReference)},
		}
		scan.walk(root, "common", 0, false)
		scan.walk(root, "events", 0, false)
		if _, err := root.Lstat("dlc"); err == nil || !os.IsNotExist(err) {
			scan.walk(root, "dlc", 0, true)
		}
		if scan.files == 0 {
			scan.index.Status = "unavailable"
		} else if scan.partial || ctx.Err() != nil {
			scan.index.Status = "partial"
		}
		fmt.Fprintf(scan.digest, "flag-scripts-v1:%s", scan.index.Status)
		scan.index.Fingerprint = hex.EncodeToString(scan.digest.Sum(nil))
		return scan.index
	})
}

func flagGameRoot(localizationDir string) *os.Root {
	if localizationDir == "" {
		return nil
	}
	abs, err := filepath.Abs(localizationDir)
	if err != nil || strings.HasPrefix(filepath.VolumeName(abs), `\\`) || !strings.EqualFold(filepath.Base(filepath.Dir(abs)), "localisation") {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil || strings.HasPrefix(filepath.VolumeName(resolved), `\\`) || !strings.EqualFold(filepath.Base(filepath.Dir(resolved)), "localisation") {
		return nil
	}
	root, err := os.OpenRoot(filepath.Dir(filepath.Dir(resolved)))
	if err != nil {
		return nil
	}
	for _, name := range []string{"common", "events", filepath.Base(filepath.Dir(resolved)), filepath.Join(filepath.Base(filepath.Dir(resolved)), filepath.Base(resolved))} {
		info, err := root.Lstat(name)
		if err != nil || !info.IsDir() {
			root.Close()
			return nil
		}
	}
	regular := func(name string) bool {
		info, err := root.Lstat(name)
		return err == nil && info.Mode().IsRegular()
	}
	if !regular("launcher-settings.json") || (!regular("stellaris.exe") && !regular("stellaris")) {
		root.Close()
		return nil
	}
	return root
}

type flagScriptScan struct {
	ctx                context.Context
	digest             hash.Hash
	index              FlagScriptIndex
	bytes              int64
	walkEntries, files int
	archives           int
	partial            bool
}

func (s *flagScriptScan) walk(root *os.Root, dir string, depth int, archives bool) {
	if s.ctx.Err() != nil || depth > 16 || s.walkEntries >= maxFlagWalkEntries || s.bytes >= maxFlagScanBytes {
		s.partial = true
		return
	}
	info, err := root.Lstat(dir)
	if err != nil || !info.IsDir() {
		s.partial = true
		return
	}
	f, err := root.Open(dir)
	if err != nil {
		s.partial = true
		return
	}
	remaining := maxFlagWalkEntries - s.walkEntries
	entries, err := f.ReadDir(remaining + 1)
	f.Close()
	if (err != nil && err != io.EOF) || len(entries) > remaining {
		s.partial = true
		return
	}
	s.walkEntries += len(entries)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	extension := ".txt"
	if archives {
		extension = ".zip"
	}
	for _, entry := range entries {
		if s.ctx.Err() != nil || s.bytes >= maxFlagScanBytes {
			s.partial = true
			return
		}
		name := path.Join(dir, entry.Name())
		if entry.IsDir() {
			s.walk(root, name, depth+1, archives)
		} else if entry.Type()&os.ModeSymlink != 0 {
			s.partial = true
		} else if strings.EqualFold(path.Ext(name), extension) {
			s.readFile(root, name, archives)
		}
	}
}

func (s *flagScriptScan) readFile(root *os.Root, name string, archive bool) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		s.partial = true
		return
	}
	f, err := root.Open(name)
	if err != nil {
		s.partial = true
		return
	}
	defer f.Close()
	if archive {
		s.readZip(f, name, info.Size())
	} else {
		s.readScript(f, name, info.Size())
	}
}

func (s *flagScriptScan) readZip(file *os.File, name string, size int64) {
	if s.archives >= maxFlagArchives || size < 22 || size > maxFlagArchiveBytes || !flagZipDirectoryBounded(file, size) {
		s.partial = true
		return
	}
	s.archives++
	reader := &flagZipReadAt{ReaderAt: file, ctx: s.ctx, remaining: maxFlagZipDirectory + 2*65*1024}
	z, err := zip.NewReader(reader, size)
	if err != nil || len(z.File) > maxFlagZipMembers {
		s.partial = true
		return
	}
	reader.remaining = maxFlagScanBytes
	names := make(map[string]int, len(z.File))
	for _, entry := range z.File {
		names[entry.Name]++
	}
	for _, entry := range z.File {
		if s.ctx.Err() != nil || s.bytes >= maxFlagScanBytes {
			s.partial = true
			return
		}
		if entry.FileInfo().IsDir() || !strings.EqualFold(path.Ext(entry.Name), ".txt") {
			continue
		}
		if !fs.ValidPath(entry.Name) || strings.ContainsAny(entry.Name, "\\:") || names[entry.Name] != 1 ||
			!entry.Mode().IsRegular() || entry.UncompressedSize64 > maxFlagScriptBytes {
			s.partial = true
			continue
		}
		content, err := entry.Open()
		if err != nil {
			s.partial = true
			continue
		}
		s.readScript(content, name+":"+entry.Name, int64(entry.UncompressedSize64))
		content.Close()
	}
}

// Reject unbounded central directories before archive/zip allocates its file list.
func flagZipDirectoryBounded(reader io.ReaderAt, size int64) bool {
	tail := make([]byte, min(size, 65*1024))
	if _, err := reader.ReadAt(tail, size-int64(len(tail))); err != nil {
		return false
	}
	for i := len(tail) - 22; i >= 0; i-- {
		p := tail[i:]
		if string(p[:4]) != "PK\x05\x06" || int(binary.LittleEndian.Uint16(p[20:])) > len(p)-22 {
			continue
		}
		count := binary.LittleEndian.Uint16(p[10:])
		directorySize := int64(binary.LittleEndian.Uint32(p[12:]))
		start := int64(binary.LittleEndian.Uint32(p[16:]))
		return binary.LittleEndian.Uint32(p[4:]) == 0 && binary.LittleEndian.Uint16(p[8:]) == count &&
			count <= maxFlagZipMembers && directorySize != 0xffff && directorySize <= maxFlagZipDirectory &&
			start+directorySize == size-int64(len(p))
	}
	return false
}

type flagZipReadAt struct {
	io.ReaderAt
	ctx       context.Context
	remaining int64
}

func (r *flagZipReadAt) ReadAt(p []byte, offset int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > r.remaining {
		return 0, io.ErrUnexpectedEOF
	}
	n, err := r.ReaderAt.ReadAt(p, offset)
	r.remaining -= int64(n)
	return n, err
}

func (s *flagScriptScan) readScript(r io.Reader, name string, size int64) {
	limit := min(int64(maxFlagScriptBytes), maxFlagScanBytes-s.bytes)
	if size < 0 || size > limit || s.ctx.Err() != nil {
		s.partial = true
		return
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	s.bytes += int64(len(data))
	if err != nil || int64(len(data)) > limit || s.ctx.Err() != nil {
		s.partial = true
		return
	}
	s.files++
	fmt.Fprintf(s.digest, "%d:%s:%x\n", len(name), name, sha256.Sum256(data))
	for _, command := range flagScriptCommands {
		if bytes.Contains(data, []byte(command)) {
			s.indexScript(data)
			return
		}
	}
}

func (s *flagScriptScan) indexScript(data []byte) {
	root, err := clausewitz.ParseRaw(s.ctx, data)
	if err != nil {
		s.partial = true
		return
	}
	counts := make(map[string][len(flagScriptCommands)]int)
	stack := []*clausewitz.RawNode{root}
	for len(stack) > 0 {
		if s.ctx.Err() != nil {
			s.partial = true
			return
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.Key == nil {
			stack = append(stack, n.Children...)
			continue
		}
		recognized := false
		for i, command := range flagScriptCommands {
			if n.Key.Text != command {
				continue
			}
			recognized = true
			key, ok := flagScriptLiteral(n, command == "set_timed_global_flag")
			if !ok {
				s.partial = true
				break
			}
			if _, exists := counts[key]; !exists && len(counts) >= maxFlagScriptKeys {
				s.partial = true
				break
			}
			values := counts[key]
			values[i]++
			counts[key] = values
			break
		}
		if !recognized {
			stack = append(stack, n.Children...)
		}
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		refs, exists := s.index.byKey[key]
		if !exists && len(s.index.byKey) >= maxFlagScriptKeys {
			s.partial = true
			continue
		}
		for i, count := range counts[key] {
			if count > 0 {
				refs[i].Sites += count
				refs[i].Files++
			}
		}
		s.index.byKey[key] = refs
	}
}

func flagScriptLiteral(node *clausewitz.RawNode, timed bool) (string, bool) {
	value := node.Scalar
	if timed {
		value = nil
		if node.Kind != "object" {
			return "", false
		}
		for _, field := range node.Children {
			if field.Key.Text == "flag" {
				if value != nil || field.Scalar == nil {
					return "", false
				}
				value = field.Scalar
			}
		}
	}
	if value == nil || len(value.Text) == 0 || len(value.Text) > maxFlagScriptKeyBytes ||
		strings.ContainsAny(value.Text, "$@[]") || strings.HasPrefix(value.Text, "event_target:") || strings.HasPrefix(value.Text, "scope:") {
		return "", false
	}
	return value.Text, true
}
