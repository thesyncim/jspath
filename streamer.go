package gojq

import (
	"bytes"
	"encoding/json"
	"github.com/gobwas/glob"
	"io"
	"strings"
)

// A PathDecoder reads and decodes JSON values from an input stream at specified path.
type PathDecoder struct {
	r       io.Reader
	buf     []byte
	scanp   int   // start of unread data in buf
	scanned int64 // amount of data already scanned
	scan    scanner
	err     error

	useNumber             bool
	disallowUnknownFields bool

	tokenState int
	tokenStack []int

	path *PathBuilder
}

type matcher func(d *PathBuilder, jsPath string) bool

// NewDecoder returns a new decoder that reads from r.
//
// The decoder introduces its own buffering and may
// read data from r beyond the JSON values requested.
func NewDecoder(r io.Reader) *PathDecoder {
	return &PathDecoder{r: r, path: NewPathBuilder()}
}

// UseNumber causes the PathDecoder to unmarshal a number into an interface{} as a
// Number instead of as a float64.
func (dec *PathDecoder) UseNumber() { dec.useNumber = true }

// DisallowUnknownFields causes the PathDecoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func (dec *PathDecoder) DisallowUnknownFields() { dec.disallowUnknownFields = true }

// Decode reads the next JSON-encoded value from its
// input and stores it in the value pointed to by v.
//
// See the documentation for Unmarshal for details about
// the conversion of JSON into a Go value.
func (dec *PathDecoder) Decode(v interface{}) error {
	if dec.err != nil {
		return dec.err
	}

	if err := dec.tokenPrepareForDecode(); err != nil {
		return err
	}

	if !dec.tokenValueAllowed() {
		return &SyntaxError{msg: "not at beginning of value", Offset: dec.offset()}
	}

	// Read whole value into buffer.
	n, err := dec.readValue()
	if err != nil {
		return err
	}

	rdr := dec.buf[dec.scanp : dec.scanp+n]
	dec.scanp += n

	// Don't save err from unmarshal into dec.err:
	// the connection is still usable since we read a complete JSON
	// object from it before the error happened.
	err = json.Unmarshal(rdr, v)

	// fixup token streaming state
	dec.tokenValueEnd()

	return err
}

func (dec *PathDecoder) DecodeBytes() ([]byte, error) {
	if dec.err != nil {
		return nil, dec.err
	}

	if err := dec.tokenPrepareForDecode(); err != nil {
		return nil, err
	}

	if !dec.tokenValueAllowed() {
		return nil, &SyntaxError{msg: "not at beginning of value", Offset: dec.offset()}
	}

	// Read whole value into buffer.
	n, err := dec.readValue()
	if err != nil {
		return nil, err
	}

	out := dec.buf[dec.scanp : dec.scanp+n]

	dec.scanp += n

	// Don't save err from unmarshal into dec.err:
	// the connection is still usable since we read a complete JSON
	// object from it before the error happened.
	//err = json.Unmarshal(rdr, v)

	// fixup token streaming state
	dec.tokenValueEnd()

	return out, err
}

// Buffered returns a reader of the data remaining in the PathDecoder's
// buffer. The reader is valid until the next call to Decode.
func (dec *PathDecoder) Buffered() io.Reader {
	return bytes.NewReader(dec.buf[dec.scanp:])
}

// readValue reads a JSON value into dec.buf.
// It returns the length of the encoding.
func (dec *PathDecoder) readValue() (int, error) {
	dec.scan.reset()

	scanp := dec.scanp
	var err error
Input:
	for {
		// Look in the buffer for a new value.
		for i, c := range dec.buf[scanp:] {
			dec.scan.bytes++
			v := dec.scan.step(&dec.scan, c)
			if v == scanEnd {
				scanp += i
				break Input
			}
			// scanEnd is delayed one byte.
			// We might block trying to get that byte from src,
			// so instead invent a space byte.
			if (v == scanEndObject || v == scanEndArray) && dec.scan.step(&dec.scan, ' ') == scanEnd {
				scanp += i + 1
				break Input
			}
			if v == scanError {
				dec.err = dec.scan.err
				return 0, dec.scan.err
			}
		}
		scanp = len(dec.buf)

		// Did the last read have an error?
		// Delayed until now to allow buffer scan.
		if err != nil {
			if err == io.EOF {
				if dec.scan.step(&dec.scan, ' ') == scanEnd {
					break Input
				}
				if nonSpace(dec.buf) {
					err = io.ErrUnexpectedEOF
				}
			}
			dec.err = err
			return 0, err
		}

		n := scanp - dec.scanp
		err = dec.refill()
		scanp = dec.scanp + n
	}
	return scanp - dec.scanp, nil
}

func (dec *PathDecoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if dec.scanp > 0 {
		dec.scanned += int64(dec.scanp)
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}

	// Grow buffer if not large enough.
	const minRead = 4096
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		dec.buf = newBuf
	}

	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[0 : len(dec.buf)+n]

	return err
}

func nonSpace(b []byte) bool {
	for _, c := range b {
		if !isSpace(c) {
			return true
		}
	}
	return false
}

// A Token holds a value of one of these types:
//
//	Delim, for the four JSON delimiters [ ] { }
//	bool, for JSON booleans
//	float64, for JSON numbers
//	Number, for JSON numbers
//	string, for JSON string literals
//	nil, for JSON null
//
type Token interface{}

const (
	tokenTopValue = iota
	tokenArrayStart
	tokenArrayValue
	tokenArrayComma
	tokenObjectStart
	tokenObjectKey
	tokenObjectColon
	tokenObjectValue
	tokenObjectComma
)

// advance tokenstate from a separator state to a value state
func (dec *PathDecoder) tokenPrepareForDecode() error {
	// Note: Not calling Peek before switch, to avoid
	// putting Peek into the standard Decode path.
	// Peek is only called when using the Token API.
	switch dec.tokenState {
	case tokenArrayComma:
		c, err := dec.Peek()
		if err != nil {
			return err
		}
		if c != ',' {
			return &SyntaxError{msg: "expected comma after array element", Offset: dec.offset()}
		}
		dec.scanp++
		dec.tokenState = tokenArrayValue
	case tokenObjectColon:
		c, err := dec.Peek()
		if err != nil {
			return err
		}
		if c != ':' {
			return &SyntaxError{msg: "expected colon after object key", Offset: dec.offset()}
		}
		dec.scanp++
		dec.tokenState = tokenObjectValue
	}
	return nil
}

func (dec *PathDecoder) tokenValueAllowed() bool {
	switch dec.tokenState {
	case tokenTopValue, tokenArrayStart, tokenArrayValue, tokenObjectValue:
		return true
	}
	return false
}

func (dec *PathDecoder) tokenValueEnd() {
	switch dec.tokenState {
	case tokenArrayStart, tokenArrayValue:
		dec.tokenState = tokenArrayComma
	case tokenObjectValue:
		dec.tokenState = tokenObjectComma
	}
}

// A Delim is a JSON array or object delimiter, one of [ ] { or }.
type Delim rune

func (d Delim) String() string {
	return string(d)
}

// Token returns the next JSON token in the input stream.
// At the end of the input stream, Token returns nil, io.EOF.
//
// Token guarantees that the delimiters [ ] { } it returns are
// properly nested and matched: if Token encounters an unexpected
// delimiter in the input, it will return an error.
//
// The input stream consists of basic JSON values—bool, string,
// number, and null—along with delimiters [ ] { } of type Delim
// to mark the start and end of arrays and objects.
// Commas and colons are elided.
func (dec *PathDecoder) Token() (Token, error) {
	for {
		c, err := dec.Peek()
		if err != nil {
			return nil, err
		}
		switch c {
		case '[':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenArrayStart
			return Delim('['), nil

		case ']':
			if dec.tokenState != tokenArrayStart && dec.tokenState != tokenArrayComma {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim(']'), nil

		case '{':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenObjectStart
			return Delim('{'), nil

		case '}':
			if dec.tokenState != tokenObjectStart && dec.tokenState != tokenObjectComma {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim('}'), nil

		case ':':
			if dec.tokenState != tokenObjectColon {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = tokenObjectValue
			continue

		case ',':
			if dec.tokenState == tokenArrayComma {
				dec.scanp++
				dec.tokenState = tokenArrayValue
				continue
			}
			if dec.tokenState == tokenObjectComma {
				dec.scanp++
				dec.tokenState = tokenObjectKey
				continue
			}
			return dec.tokenError(c)

		case '"':
			if dec.tokenState == tokenObjectStart || dec.tokenState == tokenObjectKey {
				var x string
				old := dec.tokenState
				dec.tokenState = tokenTopValue
				err := dec.Decode(&x)
				dec.tokenState = old
				if err != nil {
					return nil, err
				}
				dec.tokenState = tokenObjectColon
				return x, nil
			}
			fallthrough

		default:
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			var x interface{}
			if err := dec.Decode(&x); err != nil {
				return nil, err
			}

			return x, nil
		}
	}
}

func (dec *PathDecoder) DecodePath(path string, decodeFunc func(message json.RawMessage) error) (err error) {
	matcher, err := compilePath(path)
	if err != nil {
		return err
	}
	return dec.decodePath(path, decodeFunc, matcher)
}

func (dec *PathDecoder) decodePath(jsPath string, decodeFunc func(message json.RawMessage) error, matchPath matcher) (err error) {
	for {
		c, err := dec.Peek()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch c {
		case '[':
			if !dec.tokenValueAllowed() {
				return dec.tokenError2(c)
			}
			match := matchPath(dec.path, jsPath)
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenArrayStart
			if dec.More() {
				dec.path.StartArray()

				if match {
					for {
						if !dec.More() {
							break
						}
						bytes, err := dec.DecodeBytes()
						if err != nil {
							return err
						}
						if err := decodeFunc(bytes); err != nil {
							return err
						}
					}
				}
			}
			continue
		case ']':
			if dec.tokenState != tokenArrayStart && dec.tokenState != tokenArrayComma {
				return dec.tokenError2(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.path.EndObject()
			dec.tokenValueEnd()
			continue

		case '{':
			if !dec.tokenValueAllowed() {
				return dec.tokenError2(c)
			}
			match := matchPath(dec.path, jsPath)
			if dec.More() {
				if match {
					bytes, err := dec.DecodeBytes()
					if err != nil {
						return err
					}
					if err := decodeFunc(bytes); err != nil {
						return err
					}
					continue
				}
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.path.StartObject()
			dec.tokenState = tokenObjectStart

			continue

		case '}':
			if dec.tokenState != tokenObjectStart && dec.tokenState != tokenObjectComma {
				return dec.tokenError2(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.path.EndObject()

			dec.tokenValueEnd()
			continue
		case ':':
			if dec.tokenState != tokenObjectColon {
				return dec.tokenError2(c)
			}
			dec.scanp++
			dec.tokenState = tokenObjectValue
			continue

		case ',':
			if dec.tokenState == tokenArrayComma {
				dec.scanp++
				dec.path.IncrementArrayIndex()
				dec.tokenState = tokenArrayValue
				continue
			}
			if dec.tokenState == tokenObjectComma {
				dec.scanp++
				dec.tokenState = tokenObjectKey
				continue
			}
			return dec.tokenError2(c)

		case '"':
			if dec.tokenState == tokenObjectStart || dec.tokenState == tokenObjectKey {
				old := dec.tokenState
				dec.tokenState = tokenTopValue
				keyBytes, err := dec.DecodeBytes()
				dec.tokenState = old
				if err != nil {
					return err
				}
				dec.path.SetObjectKey(keyBytes[1 : len(keyBytes)-1])
				dec.tokenState = tokenObjectColon
				continue
			}
			fallthrough

		default:
			if !dec.tokenValueAllowed() {
				return dec.tokenError2(c)
			}

			if bytes, err := dec.DecodeBytes(); err != nil {
				return err
			} else {
				if matchPath(dec.path, jsPath) {
					if err := decodeFunc(bytes); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (dec *PathDecoder) tokenError(c byte) (Token, error) {
	var context string
	switch dec.tokenState {
	case tokenTopValue:
		context = " looking for beginning of value"
	case tokenArrayStart, tokenArrayValue, tokenObjectValue:
		context = " looking for beginning of value"
	case tokenArrayComma:
		context = " after array element"
	case tokenObjectKey:
		context = " looking for beginning of object key string"
	case tokenObjectColon:
		context = " after object key"
	case tokenObjectComma:
		context = " after object key:value pair"
	}
	_ = context
	return nil, &SyntaxError{msg: "invalid character ", Offset: dec.offset()} //+ quoteChar(c) + " " + context, dec.offset()}
}

func (dec *PathDecoder) tokenError2(c byte) error {
	var context string
	switch dec.tokenState {
	case tokenTopValue:
		context = " looking for beginning of value"
	case tokenArrayStart, tokenArrayValue, tokenObjectValue:
		context = " looking for beginning of value"
	case tokenArrayComma:
		context = " after array element"
	case tokenObjectKey:
		context = " looking for beginning of object key string"
	case tokenObjectColon:
		context = " after object key"
	case tokenObjectComma:
		context = " after object key:value pair"
	}
	_ = context
	return &SyntaxError{msg: "invalid character ", Offset: dec.offset()} //+ quoteChar(c) + " " + context, dec.offset()}
}

// More reports whether there is another element in the
// current array or object being parsed.
func (dec *PathDecoder) More() bool {
	c, err := dec.Peek()
	return err == nil && c != ']' && c != '}'
}

func (dec *PathDecoder) Peek() (byte, error) {
	var err error
	for {
		for i := dec.scanp; i < len(dec.buf); i++ {
			c := dec.buf[i]
			if isSpace(c) {
				continue
			}
			dec.scanp = i
			return c, nil
		}
		// buffer has been scanned, now report any error
		if err != nil {
			return 0, err
		}
		err = dec.refill()
	}
}

func (dec *PathDecoder) offset() int64 {
	return dec.scanned + int64(dec.scanp)
}

func compilePath(jsPath string) (matcher, error) {
	if i := strings.IndexByte(jsPath, '*'); i != -1 {
		s := escapeGlob(jsPath)
		re, err := glob.Compile(s)
		if err != nil {
			return nil, err
		}
		return func(d *PathBuilder, jsPath string) bool {
			return re.Match(d.Path())
		}, nil
	}
	return func(d *PathBuilder, jsPath string) bool {
		return d.MatchString(jsPath)
	}, nil
}

func escapeGlob(jsPath string) string {
	s := strings.Replace(jsPath, "[", "\\[", -1)
	s = strings.Replace(s, "]", "\\]", -1)
	return s
}
