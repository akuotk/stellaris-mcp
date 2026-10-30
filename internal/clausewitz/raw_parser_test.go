package clausewitz

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseRawMixedOrderedEntriesAndExactRanges(t *testing.T) {
	data := []byte("\xef\xbb\xbf# comment\r\nouter={a=one plain {child=yes} a=\"two\\nlines\" \"quoted key\"=\"escaped\\\"quote\" empty={} nested={first x=2 {last=three}}} # ending")
	root, err := ParseRaw(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	if root.Start != 0 || root.End != len(data) || len(root.Children) != 1 {
		t.Fatal("document range or children lost")
	}
	block := root.Children[0]
	if block.Kind != "mixed" || len(block.Children) != 7 {
		t.Fatal("mixed entry count/order lost")
	}
	if string(data[block.Start:block.End])[0] != '{' || string(data[block.EntryStart:block.Start]) != "outer=" {
		t.Fatal("block range excludes original syntax")
	}
	if block.Children[1].Key != nil || block.Children[1].Scalar.Text != "plain" || block.Children[2].Key != nil {
		t.Fatal("standalone entries lost")
	}
	quoted := block.Children[3]
	if quoted.Key.Text != "a" || quoted.Scalar.Text != "two\nlines" || !quoted.Scalar.Quoted || !quoted.Scalar.Escaped || string(data[quoted.Start:quoted.End]) != `"two\nlines"` {
		t.Fatal("quoted token ranges or normalization wrong")
	}
	if !block.Children[4].Key.Quoted || block.Children[4].Scalar.Text != `escaped"quote` {
		t.Fatal("quoted key or escaped quote lost")
	}
	if block.Children[5].Kind != "empty_block" || block.Children[6].Kind != "mixed" {
		t.Fatal("empty/nested mixed block kinds wrong")
	}
}

func TestParseRawTokenSpellingsAndMalformedInput(t *testing.T) {
	root, err := ParseRaw(context.Background(), []byte(`bare=12 quoted="12" blank="" unknown="keep\q" list={a b} ""=yes`))
	if err != nil {
		t.Fatal(err)
	}
	if root.Children[0].Scalar.Quoted || !root.Children[1].Scalar.Quoted || root.Children[0].Scalar.Text != root.Children[1].Scalar.Text {
		t.Fatal("quoted/unquoted distinction lost")
	}
	if root.Children[2].Scalar.Text != "" || root.Children[3].Scalar.Text != `keep\q` || root.Children[4].Kind != "list" || root.Children[5].Key.Text != "" {
		t.Fatal("empty or unknown-escape token changed")
	}
	for _, input := range []string{`a=`, `a==b`, `a={b=c`, `a={b=}`, `}`, `a="unterminated`, `a="trailing\`} {
		if _, err := ParseRaw(context.Background(), []byte(input)); err == nil {
			t.Fatalf("malformed input accepted: %q", input)
		}
	}
}

func TestParseRawDeepNestingAndCancellation(t *testing.T) {
	depth := 10000
	input := []byte(strings.Repeat("nested={", depth) + "leaf=yes" + strings.Repeat("}", depth))
	root, err := ParseRaw(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	node := root
	for i := 0; i <= depth; i++ {
		if len(node.Children) != 1 {
			t.Fatal("nested node lost")
		}
		node = node.Children[0]
	}
	if node.Scalar.Text != "yes" {
		t.Fatal("deep leaf lost")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ParseRaw(ctx, input); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
}

type cancelDuringRawToken struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *cancelDuringRawToken) Err() error {
	c.checks++
	if c.checks >= 4 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestRawLexerCancellationWithinEscapedToken(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &cancelDuringRawToken{Context: base, cancel: cancel}
	data := []byte(`"` + strings.Repeat(`\n`, 100000) + `"`)
	lex := rawLexer{ctx: ctx, data: data}
	if _, err := lex.scan(); !errors.Is(err, context.Canceled) {
		t.Fatal("long escaped token ignored cancellation")
	}
	if lex.pos < 1000 || lex.pos >= len(data) {
		t.Fatal("cancellation was not checked within the token")
	}
}
