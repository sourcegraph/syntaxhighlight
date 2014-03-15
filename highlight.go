// Package syntaxhighlight provides syntax highlighting for code. It currently
// uses a language-independent lexer and performs decently on JavaScript, Java,
// Ruby, Python, Go, and C.
package syntaxhighlight

import (
	"bufio"
	"bytes"
	"github.com/sourcegraph/annotate"
	"io"
	"text/template"
	"unicode"
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
	Print(w io.Writer, tok []byte, kind int) error
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

func (p HTMLPrinter) Print(w io.Writer, tok []byte, kind int) error {
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
	template.HTMLEscape(w, tok)
	if class != "" {
		_, err := w.Write([]byte(`</span>`))
		if err != nil {
			return err
		}
	}
	return nil
}

type Annotator interface {
	Annotate(start int, tok []byte, kind int) (*annotate.Annotation, error)
}

type HTMLAnnotator HTMLConfig

func (a HTMLAnnotator) Annotate(start int, tok []byte, kind int) (*annotate.Annotation, error) {
	class := ((HTMLConfig)(a)).class(kind)
	if class != "" {
		left := []byte(`<span class="`)
		left = append(left, []byte(class)...)
		left = append(left, []byte(`">`)...)
		return &annotate.Annotation{
			Start: start, End: start + len([]rune(string(tok))),
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

func Print(s *Scanner, w io.Writer, p Printer) error {
	for s.Scan() {
		tok, kind := s.Token()
		err := p.Print(w, tok, kind)
		if err != nil {
			return err
		}
	}

	if err := s.Err(); err != nil {
		return err
	}

	return nil
}

func Annotate(src []byte, a Annotator) ([]*annotate.Annotation, error) {
	s := NewScanner(src)

	var anns []*annotate.Annotation
	read := 0
	for s.Scan() {
		tok, kind := s.Token()
		ann, err := a.Annotate(read, tok, kind)
		if err != nil {
			return nil, err
		}
		read += len([]rune(string(tok)))
		if ann != nil {
			anns = append(anns, ann)
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
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

type Scanner struct {
	*bufio.Scanner
	kind int
	typ  bool
	name bool
}

func NewScanner(src []byte) *Scanner {
	r := bytes.NewReader(src)
	s := &Scanner{Scanner: bufio.NewScanner(r)}
	s.Split(func(byteData []byte, atEOF bool) (advance int, token []byte, err error) {
		data := []rune(string(byteData))
		if atEOF && len(byteData) == 0 {
			return 0, nil, nil
		}

		//		r, _ := utf8.DecodeRune(data)
		r := data[0]

		retRunes := func(data []rune) (int, []byte, error) {
			seg := []byte(string(data))
			return len(seg), seg, nil
		}

		retBytes := func() (int, []byte, error) {
			return len(byteData), byteData, nil
		}

		isQuot := func(r rune) bool {
			return r == '`' || r == '\'' || r == '"'
		}

		if isQuot(r) {
			s.kind = STRING
			for j := 1; j < len(data); j++ {
				if data[j] == '\\' {
					j++
				} else if data[j] == r {
					return retRunes(data[0 : j+1])
				} else if atEOF {
					return retBytes()
				}
			}
			return 0, nil, nil
		}

		alpha := func(r rune) bool {
			return byte(r) == '_' || unicode.IsLetter(r)
		}
		alnum := func(r rune) bool {
			return alpha(r) || unicode.IsDigit(r)
		}

		if unicode.IsUpper(r) {
			s.typ = true
			s.kind = TYPE
		} else if alpha(r) {
			s.name = true
			s.kind = PLAINTEXT
		}
		if s.typ || s.name {
			i := lastContiguousIndexFunc([]byte(string(data)), alnum)
			if i >= 0 {
				s.typ, s.name = false, false
				if _, isKwd := Keywords[string(data[0:i+1])]; isKwd {
					s.kind = KEYWORD
				}
				return retRunes(data[0 : i+1])
			}
			return 0, nil, nil
		}

		if unicode.IsDigit(r) {
			s.kind = DECIMAL
			i := lastContiguousIndexFunc([]byte(string(data)), unicode.IsDigit)
			if i >= 0 {
				return retRunes(data[:i+1])
			}
			return 0, nil, nil
		}

		if unicode.IsSpace(r) {
			s.kind = WHITESPACE
			i := lastContiguousIndexFunc([]byte(string(data)), unicode.IsSpace)
			if i >= 0 {
				if i+1 <= len(data) {
					return retRunes(data[:i+1])
				} else {
					return retRunes(data)
				}
			}
			if atEOF {
				return retBytes()
			}
			return 0, nil, nil
		}

		lineComments := []string{"//", "#"}
		for _, lc := range lineComments {
			if i := indexRunes(data, string(lc)); i == 0 {
				s.kind = COMMENT
				if i := indexRune(data, '\n'); i >= 0 {
					return retRunes(data[0 : i+1])
				}
				if atEOF {
					return retBytes()
				}
				return 0, nil, nil
			}
		}

		if i := indexRunes(data, "/*"); i == 0 {
			s.kind = COMMENT
			if i := indexRunes(data, "*/"); i >= 0 {
				return retRunes(data[0 : i+2])
			}
			if atEOF {
				return retBytes()
			}
			return 0, nil, nil
		}

		if i := bytes.IndexFunc([]byte(string(data)), func(r rune) bool { return !alnum(r) && !unicode.IsSpace(r) && !isQuot(r) }); i >= 0 {
			s.kind = PUNCTUATION
			return retRunes(data[0 : i+1])
		}

		if atEOF {
			return retBytes()
		}

		return 0, nil, nil
	})
	return s
}

func lastContiguousIndexFunc(s []byte, f func(r rune) bool) int {
	i := bytes.IndexFunc(s, func(r rune) bool {
		return !f(r)
	})
	if i == -1 {
		i = len(s)
	}
	return i - 1
}

func (s *Scanner) Token() ([]byte, int) {
	return s.Bytes(), s.kind
}

func indexRune(src []rune, c rune) int {
	for i, r := range src {
		if r == c {
			return i
		}
	}
	return -1
}

func indexRunes(src []rune, c string) int {
	l := len(src)
	w := len([]rune(c))
	for i := 0; i < l-w; i++ {
		if string(src[i:i+w]) == c {
			return i
		}
	}
	return -1
}
