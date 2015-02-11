// Package syntaxhighlight provides syntax highlighting for code. It currently
// uses a language-independent lexer and performs decently on JavaScript, Java,
// Ruby, Python, Go, and C.
package syntaxhighlight

import (
	"bytes"
	"html"
	"io"
	"strings"
	"text/scanner"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/sourcegraph/annotate"
	"sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"sourcegraph.com/sourcegraph/vcsstore/vcsclient"
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

// NilAnnotator is a special kind of annotator that always returns nil, but stores
// within itself the snippet of source code that is passed through it as tokens.
//
// This functionality is useful when one wishes to obtain the tokenized source as a data
// structure, as opposed to an annotated string, allowing full control over rendering and
// displaying it.
type NilAnnotator struct {
	Config     HTMLConfig
	Code       *sourcegraph.SourceCode
	byteOffset int
}

func NewNilAnnotator(e *vcsclient.FileWithRange) *NilAnnotator {
	ann := NilAnnotator{
		Config: DefaultHTMLConfig,
		Code: &sourcegraph.SourceCode{
			Lines: make([]*sourcegraph.SourceCodeLine, 0, bytes.Count(e.Contents, []byte("\n"))),
		},
		byteOffset: e.StartByte,
	}
	ann.addLine(ann.byteOffset)
	return &ann
}

func (a *NilAnnotator) addToken(t interface{}) {
	line := a.Code.Lines[len(a.Code.Lines)-1]
	if line.Tokens == nil {
		line.Tokens = make([]interface{}, 0, 1)
	}
	// If this token and the previous one are both strings, merge them.
	n := len(line.Tokens)
	if t1, ok := t.(string); ok && n > 0 {
		if t2, ok := (line.Tokens[n-1]).(string); ok {
			line.Tokens[n-1] = string(t1 + t2)
			return
		}
	}
	line.Tokens = append(line.Tokens, t)
}

func (a *NilAnnotator) addLine(startByte int) {
	a.Code.Lines = append(a.Code.Lines, &sourcegraph.SourceCodeLine{StartByte: startByte})
	if len(a.Code.Lines) > 1 {
		lastLine := a.Code.Lines[len(a.Code.Lines)-2]
		lastLine.EndByte = startByte - 1
	}
}

func (a *NilAnnotator) addMultilineComment(startByte int, text string) {
	lines := strings.Split(text, "\n")
	for n, text := range lines {
		if len(text) > 0 {
			a.addToken(&sourcegraph.SourceCodeToken{
				StartByte: startByte,
				EndByte:   startByte + len(text),
				Class:     "com",
				Label:     text,
			})
			startByte += len(text)
		}
		if n < len(lines)-1 {
			a.addLine(startByte)
		}
	}
}

func (a *NilAnnotator) Annotate(start, kind int, tokText string) (*annotate.Annotation, error) {
	class := ((HTMLConfig)(a.Config)).class(kind)
	txt := html.EscapeString(tokText)
	start += a.byteOffset

	switch {
	// New line char
	case tokText == "\n":
		a.addLine(start + 1)

	// Whitespace
	case class == "":
		a.addToken(txt)

	// Multiline comment
	case class == "com" && strings.Contains(tokText, "\n"):
		a.addMultilineComment(start+1, tokText)

	// Token
	default:
		a.addToken(&sourcegraph.SourceCodeToken{
			StartByte: start,
			EndByte:   start + len(tokText),
			Class:     class,
			Label:     txt,
		})
	}

	return nil, nil
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

func Annotate(src []byte, a Annotator) (annotate.Annotations, error) {
	s := NewScanner(src)

	var anns annotate.Annotations
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
