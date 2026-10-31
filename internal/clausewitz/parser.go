package clausewitz

import "fmt"

// Value represents a Clausewitz value: scalar, object, or list.
type Value struct {
	Scalar string
	Object *Object
	List   []Value
	Mixed  bool
}

// IsScalar returns true if the value is a scalar (string, number, boolean).
func (v Value) IsScalar() bool { return !v.Mixed && v.Object == nil && v.List == nil }

// IsObject returns true if the value is a key-value block.
func (v Value) IsObject() bool { return !v.Mixed && v.Object != nil }

// IsList returns true if the value is a list of values.
func (v Value) IsList() bool { return !v.Mixed && v.List != nil }

// Object is an ordered collection of key-value pairs.
// Keys may be duplicated.
type Object struct {
	Pairs []Pair
}

// Pair is a key-value entry in an Object.
type Pair struct {
	Key   string
	Value Value
}

// Parse parses Clausewitz-formatted data into an Object.
func Parse(data []byte) (*Object, error) {
	p := &parser{lex: newLexer(data)}
	return p.parseDocument()
}

type parser struct {
	lex *lexer
}

func (p *parser) parseDocument() (*Object, error) {
	var pairs []Pair
	for p.lex.peek().kind != tokenEOF {
		pair, err := p.parsePair()
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}
	return &Object{Pairs: pairs}, nil
}

func (p *parser) parsePair() (Pair, error) {
	key := p.lex.next()
	if key.kind != tokenIdent && key.kind != tokenString {
		return Pair{}, fmt.Errorf("line %d: expected key, got %v", key.line, key.kind)
	}
	eq := p.lex.next()
	if eq.kind != tokenEquals {
		return Pair{}, fmt.Errorf("line %d: expected '=' after key %q", eq.line, key.value)
	}
	val, err := p.parseValue()
	if err != nil {
		return Pair{}, err
	}
	return Pair{Key: key.value, Value: val}, nil
}

func (p *parser) parseValue() (Value, error) {
	tok := p.lex.peek()
	switch tok.kind {
	case tokenLBrace:
		return p.parseBlock()
	case tokenIdent, tokenString:
		p.lex.next()
		return Value{Scalar: tok.value}, nil
	default:
		if tok.kind == tokenInvalid {
			return Value{}, fmt.Errorf("line %d: %s", tok.line, tok.value)
		}
		return Value{}, fmt.Errorf("line %d: expected value", tok.line)
	}
}

// parseBlock keeps mixed keyed and anonymous values distinct from valid typed objects and lists.
func (p *parser) parseBlock() (Value, error) {
	p.lex.next()
	var pairs []Pair
	var items []Value
	for {
		tok := p.lex.peek()
		switch tok.kind {
		case tokenRBrace:
			p.lex.next()
			if len(pairs) > 0 && len(items) > 0 {
				return Value{Mixed: true}, nil
			}
			if len(pairs) > 0 {
				return Value{Object: &Object{Pairs: pairs}}, nil
			}
			if len(items) > 0 {
				return Value{List: items}, nil
			}
			return Value{Object: &Object{}}, nil
		case tokenEOF:
			return Value{}, fmt.Errorf("line %d: unterminated block", tok.line)
		case tokenIdent, tokenString:
			first := p.lex.next()
			if p.lex.peek().kind == tokenEquals {
				p.lex.next()
				value, err := p.parseValue()
				if err != nil {
					return Value{}, err
				}
				pairs = append(pairs, Pair{Key: first.value, Value: value})
			} else {
				items = append(items, Value{Scalar: first.value})
			}
		case tokenLBrace:
			value, err := p.parseBlock()
			if err != nil {
				return Value{}, err
			}
			items = append(items, value)
		case tokenInvalid:
			return Value{}, fmt.Errorf("line %d: %s", tok.line, tok.value)
		default:
			return Value{}, fmt.Errorf("line %d: unexpected token in block", tok.line)
		}
	}
}
