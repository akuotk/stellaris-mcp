package clausewitz

import (
	"context"
	"fmt"
)

// RawToken retains normalized text and its exact [Start, End) source-byte range.
// Quoted and Escaped distinguish spellings that normalize to the same text.
type RawToken struct {
	Text    string
	Start   int
	End     int
	Quoted  bool
	Escaped bool
}

// RawNode keeps ordered keyed and anonymous children in the same block.
// Start/End cover the value; EntryStart/End additionally cover its optional key and '='.
// Comments and whitespace remain recoverable from these ranges and the original bytes.
type RawNode struct {
	Key        *RawToken
	Scalar     *RawToken
	Children   []*RawNode
	Kind       string
	EntryStart int
	Start      int
	End        int
}

func (n *RawNode) IsScalar() bool { return n.Scalar != nil }

// ParseRaw parses a navigable tree without the typed parser's object/list inference.
// It is iterative, so nesting does not require recursive parser calls.
func ParseRaw(ctx context.Context, data []byte) (*RawNode, error) {
	lex := rawLexer{ctx: ctx, data: data}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		lex.pos = 3
	}
	root := &RawNode{Start: 0, EntryStart: 0, End: len(data)}
	stack := []*RawNode{root}
	for {
		tok, err := lex.next()
		if err != nil {
			return nil, err
		}
		parent := stack[len(stack)-1]
		switch tok.kind {
		case rawEOF:
			if len(stack) != 1 {
				return nil, fmt.Errorf("byte %d: unterminated block opened at byte %d", len(data), parent.Start)
			}
			classifyRawBlock(root)
			return root, nil
		case rawClose:
			if len(stack) == 1 {
				return nil, fmt.Errorf("byte %d: unexpected closing brace", tok.raw.Start)
			}
			parent.End = tok.raw.End
			classifyRawBlock(parent)
			stack = stack[:len(stack)-1]
		case rawEquals:
			return nil, fmt.Errorf("byte %d: unexpected '='", tok.raw.Start)
		default:
			node := &RawNode{EntryStart: tok.raw.Start, Start: tok.raw.Start, End: tok.raw.End}
			value := tok
			if tok.kind == rawText {
				next, err := lex.peek()
				if err != nil {
					return nil, err
				}
				if next.kind == rawEquals {
					_, _ = lex.next()
					key := tok.raw
					node.Key = &key
					value, err = lex.next()
					if err != nil {
						return nil, err
					}
					if value.kind != rawText && value.kind != rawOpen {
						return nil, fmt.Errorf("byte %d: expected value after '='", value.raw.Start)
					}
					node.Start, node.End = value.raw.Start, value.raw.End
				}
			}
			parent.Children = append(parent.Children, node)
			if value.kind == rawOpen {
				stack = append(stack, node)
			} else {
				scalar := value.raw
				node.Scalar = &scalar
				node.Kind = "scalar"
			}
		}
	}
}

func classifyRawBlock(node *RawNode) {
	keyed, anonymous := false, false
	for _, child := range node.Children {
		if child.Key == nil {
			anonymous = true
		} else {
			keyed = true
		}
	}
	switch {
	case keyed && anonymous:
		node.Kind = "mixed"
	case keyed:
		node.Kind = "object"
	case anonymous:
		node.Kind = "list"
	default:
		node.Kind = "empty_block"
	}
}

const (
	rawEOF byte = iota
	rawText
	rawOpen
	rawClose
	rawEquals
)

type rawLexeme struct {
	kind byte
	raw  RawToken
}

type rawLexer struct {
	ctx     context.Context
	data    []byte
	pos     int
	checkAt int
	peeked  *rawLexeme
}

func (l *rawLexer) next() (rawLexeme, error) {
	if l.peeked != nil {
		t := *l.peeked
		l.peeked = nil
		return t, nil
	}
	return l.scan()
}
func (l *rawLexer) peek() (rawLexeme, error) {
	if l.peeked == nil {
		t, err := l.scan()
		if err != nil {
			return t, err
		}
		l.peeked = &t
	}
	return *l.peeked, nil
}

func (l *rawLexer) scan() (rawLexeme, error) {
	if err := l.ctx.Err(); err != nil {
		return rawLexeme{}, err
	}
	for l.pos < len(l.data) {
		if l.pos >= l.checkAt {
			l.checkAt = l.pos + 4096
			if err := l.ctx.Err(); err != nil {
				return rawLexeme{}, err
			}
		}
		switch l.data[l.pos] {
		case ' ', '\t', '\r', '\n':
			l.pos++
		case '#':
			for l.pos < len(l.data) && l.data[l.pos] != '\n' {
				if l.pos >= l.checkAt {
					l.checkAt = l.pos + 4096
					if err := l.ctx.Err(); err != nil {
						return rawLexeme{}, err
					}
				}
				l.pos++
			}
		default:
			goto token
		}
	}
token:
	start := l.pos
	if l.pos == len(l.data) {
		return rawLexeme{kind: rawEOF, raw: RawToken{Start: start, End: start}}, nil
	}
	switch l.data[l.pos] {
	case '{':
		l.pos++
		return rawLexeme{kind: rawOpen, raw: RawToken{Start: start, End: l.pos}}, nil
	case '}':
		l.pos++
		return rawLexeme{kind: rawClose, raw: RawToken{Start: start, End: l.pos}}, nil
	case '=':
		l.pos++
		return rawLexeme{kind: rawEquals, raw: RawToken{Start: start, End: l.pos}}, nil
	case '"':
		l.pos++
		token := RawToken{Start: start, Quoted: true}
		var decoded []byte
		contentStart := l.pos
		for l.pos < len(l.data) {
			if l.pos >= l.checkAt {
				l.checkAt = l.pos + 4096
				if err := l.ctx.Err(); err != nil {
					return rawLexeme{}, err
				}
			}
			ch := l.data[l.pos]
			if ch == '"' {
				if token.Escaped {
					decoded = append(decoded, l.data[contentStart:l.pos]...)
					token.Text = string(decoded)
				} else {
					token.Text = string(l.data[contentStart:l.pos])
				}
				l.pos++
				token.End = l.pos
				return rawLexeme{kind: rawText, raw: token}, nil
			}
			if ch == '\\' {
				if l.pos+1 >= len(l.data) {
					return rawLexeme{}, fmt.Errorf("byte %d: unterminated quoted string", start)
				}
				decoded = append(decoded, l.data[contentStart:l.pos]...)
				next := l.data[l.pos+1]
				switch next {
				case '"', '\\':
					decoded = append(decoded, next)
				case 'n':
					decoded = append(decoded, '\n')
				case 't':
					decoded = append(decoded, '\t')
				default:
					decoded = append(decoded, ch, next)
				}
				token.Escaped = true
				l.pos += 2
				contentStart = l.pos
			} else {
				l.pos++
			}
		}
		return rawLexeme{}, fmt.Errorf("byte %d: unterminated quoted string", start)
	default:
		for l.pos < len(l.data) {
			if l.pos >= l.checkAt {
				l.checkAt = l.pos + 4096
				if err := l.ctx.Err(); err != nil {
					return rawLexeme{}, err
				}
			}
			switch l.data[l.pos] {
			case ' ', '\t', '\r', '\n', '{', '}', '=', '"', '#':
				return rawLexeme{kind: rawText, raw: RawToken{Text: string(l.data[start:l.pos]), Start: start, End: l.pos}}, nil
			}
			l.pos++
		}
		return rawLexeme{kind: rawText, raw: RawToken{Text: string(l.data[start:l.pos]), Start: start, End: l.pos}}, nil
	}
}
