package syntaxhighlight

import (
	"bufio"
	"bytes"
	"io"
	"text/template"
	"unicode"
	"unicode/utf8"
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

type htmlConfig struct {
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

type htmlPrinter htmlConfig

func (p htmlPrinter) Print(w io.Writer, tok []byte, kind int) error {
	var class string
	switch kind {
	case STRING:
		class = p.String
	case KEYWORD:
		class = p.Keyword
	case COMMENT:
		class = p.Comment
	case TYPE:
		class = p.Type
	case LITERAL:
		class = p.Literal
	case PUNCTUATION:
		class = p.Punctuation
	case PLAINTEXT:
		class = p.Plaintext
	case TAG:
		class = p.Tag
	case HTMLTAG:
		class = p.HTMLTag
	case HTMLATTRNAME:
		class = p.HTMLAttrName
	case HTMLATTRVALUE:
		class = p.HTMLAttrValue
	case DECIMAL:
		class = p.Decimal
	}
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

// DefaultHTMLConfig's class names match those of
// [google-code-prettify](https://code.google.com/p/google-code-prettify/).
var DefaultHTMLConfig = htmlConfig{
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
			return nil
		}
	}

	if err := s.Err(); err != nil {
		return err
	}

	return nil
}

func AsHTML(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	err := Print(NewScanner(src), &buf, htmlPrinter(DefaultHTMLConfig))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type Scanner struct {
	*bufio.Scanner
	kind int
	quot byte
	typ  bool
	name bool
}

func NewScanner(src []byte) *Scanner {
	r := bytes.NewReader(src)
	s := &Scanner{Scanner: bufio.NewScanner(r)}
	s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if s.quot != 0 {
			for j := 1; j < len(data); {
				i := bytes.IndexByte(data[j:], s.quot)
				if i >= 0 {
					i += j
					if i > 0 && data[i-1] == '\\' {
						j += i
						continue
					}
					s.quot = 0
					return i + 1, data[0 : i+1], nil
				}
				if atEOF {
					return len(data), data, nil
				}
				return 0, nil, nil
			}
			return 0, nil, nil
		}

		r, _ := utf8.DecodeRune(data)
		if unicode.IsUpper(r) {
			s.typ = true
			s.kind = TYPE
		} else if unicode.IsLetter(r) {
			s.name = true
			s.kind = PLAINTEXT
		}
		if s.typ || s.name {
			i := lastContiguousIndexFunc(data, func(r rune) bool {
				return unicode.IsLetter(r) || unicode.IsDigit(r)
			})
			if i >= 0 {
				s.typ, s.name = false, false
				if _, isKwd := Keywords[string(data[0:i+1])]; isKwd {
					s.kind = KEYWORD
				}
				return i + 1, data[0 : i+1], nil
			}
			return 0, nil, nil
		}

		if unicode.IsDigit(r) {
			s.kind = DECIMAL
			i := lastContiguousIndexFunc(data, unicode.IsDigit)
			if i >= 0 {
				return i + 1, data[:i+1], nil
			}
			return 0, nil, nil
		}

		if unicode.IsSpace(r) {
			s.kind = WHITESPACE
			i := lastContiguousIndexFunc(data, unicode.IsSpace)
			if i >= 0 {
				return i + 1, data[:i+1], nil
			}
			if atEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		}

		if i := bytes.IndexAny(data, "{([])}<>,./+-=;:|\\!@#$%^&*\"'`"); i >= 0 {
			c := data[0]
			if c == '`' || c == '\'' || c == '"' {
				s.kind = STRING
				s.quot = c
				return 0, nil, nil
			}
			s.kind = PUNCTUATION
			return i + 1, data[0 : i+1], nil
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
