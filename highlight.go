// Package syntaxhighlight provides syntax highlighting for code. It currently
// uses a language-independent lexer and performs decently on JavaScript, Java,
// Ruby, Python, Go, and C.
package syntaxhighlight

import (
	"bytes"
	"io"
	"text/scanner"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/sourcegraph/annotate"
)

const (
	WHITESPACE = iota
	STRING
	KEYWORD
	COMMENT
	TYPE
	LITERAL
	PUNCTUATION
	PLAINTEXT
	TAG
	HTMLTAG
	HTMLATTRNAME
	HTMLATTRVALUE
	DECIMAL
)

type Printer interface {
	Print(w io.Writer, kind int, tokText string) error
}

type HTMLConfig struct {
	String        string
	Keyword       string
	Comment       string
	Type          string
	Literal       string
	Punctuation   string
	Plaintext     string
	Tag           string
	HTMLTag       string
	HTMLAttrName  string
	HTMLAttrValue string
	Decimal       string
}

type HTMLPrinter HTMLConfig

func (c HTMLConfig) class(kind int) string {
	switch kind {
	case STRING:
		return c.String
	case KEYWORD:
		return c.Keyword
	case COMMENT:
		return c.Comment
	case TYPE:
		return c.Type
	case LITERAL:
		return c.Literal
	case PUNCTUATION:
		return c.Punctuation
	case PLAINTEXT:
		return c.Plaintext
	case TAG:
		return c.Tag
	case HTMLTAG:
		return c.HTMLTag
	case HTMLATTRNAME:
		return c.HTMLAttrName
	case HTMLATTRVALUE:
		return c.HTMLAttrValue
	case DECIMAL:
		return c.Decimal
	}
	return ""
}

func (p HTMLPrinter) Print(w io.Writer, kind int, tokText string) error {
	class := ((HTMLConfig)(p)).class(kind)
	if class != "" {
		_, err := w.Write([]byte(`<span class="`))
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, class)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(`">`))
		if err != nil {
			return err
		}
	}
	template.HTMLEscape(w, []byte(tokText))
	if class != "" {
		_, err := w.Write([]byte(`</span>`))
		if err != nil {
			return err
		}
	}
	return nil
}

type Annotator interface {
	Annotate(start int, kind int, tokText string) (*annotate.Annotation, error)
}

type HTMLAnnotator HTMLConfig

func (a HTMLAnnotator) Annotate(start int, kind int, tokText string) (*annotate.Annotation, error) {
	class := ((HTMLConfig)(a)).class(kind)
	if class != "" {
		left := []byte(`<span class="`)
		left = append(left, []byte(class)...)
		left = append(left, []byte(`">`)...)
		return &annotate.Annotation{
			Start: start, End: start + len(tokText),
			Left: left, Right: []byte("</span>"),
		}, nil
	}
	return nil, nil
}

// DefaultHTMLConfig's class names match those of
// [google-code-prettify](https://code.google.com/p/google-code-prettify/).
var DefaultHTMLConfig = HTMLConfig{
	String:        "str",
	Keyword:       "kwd",
	Comment:       "com",
	Type:          "typ",
	Literal:       "lit",
	Punctuation:   "pun",
	Plaintext:     "pln",
	Tag:           "tag",
	HTMLTag:       "htm",
	HTMLAttrName:  "atn",
	HTMLAttrValue: "atv",
	Decimal:       "dec",
}

func Print(s *scanner.Scanner, w io.Writer, p Printer) error {
	tok := s.Scan()
	for tok != scanner.EOF {
		tokText := s.TokenText()
		err := p.Print(w, tokenKind(tok, tokText), tokText)
		if err != nil {
			return err
		}

		tok = s.Scan()
	}

	return nil
}

func Annotate(src []byte, a Annotator) ([]*annotate.Annotation, error) {
	s := NewScanner(src)

	var anns []*annotate.Annotation
	read := 0

	tok := s.Scan()
	for tok != scanner.EOF {
		tokText := s.TokenText()

		ann, err := a.Annotate(read, tokenKind(tok, tokText), tokText)
		if err != nil {
			return nil, err
		}
		read += len(tokText)
		if ann != nil {
			anns = append(anns, ann)
		}

		tok = s.Scan()
	}

	return anns, nil
}

func AsHTML(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	err := Print(NewScanner(src), &buf, HTMLPrinter(DefaultHTMLConfig))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func NewScanner(src []byte) *scanner.Scanner {
	var s scanner.Scanner
	s.Init(bytes.NewReader(src))
	s.Error = func(_ *scanner.Scanner, _ string) {}
	s.Whitespace = 0
	s.Mode = s.Mode ^ scanner.SkipComments
	return &s
}

func tokenKind(tok rune, tokText string) int {
	switch tok {
	case scanner.Ident:
		if _, isKW := Keywords[tokText]; isKW {
			return KEYWORD
		}
		if r, _ := utf8.DecodeRuneInString(tokText); unicode.IsUpper(r) {
			return TYPE
		}
		return PLAINTEXT
	case scanner.Float, scanner.Int:
		return DECIMAL
	case scanner.Char, scanner.String, scanner.RawString:
		return STRING
	case scanner.Comment:
		return COMMENT
	}
	if unicode.IsSpace(tok) {
		return WHITESPACE
	}
	return PUNCTUATION
}
