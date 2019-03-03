package jspath

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/gobwas/glob"
)

// A StreamDecoder reads and decodes JSON values from an input stream at specified json path.
type StreamDecoder struct {
	r       io.Reader
	buf     []byte
	scanp   int   // start of unread data in buf
	scanned int64 // amount of data already scanned
	scan    scanner
	err     error

	tokenState int
	tokenStack []int

	context context.Context

	done chan error
	path *pathBuilder
}

type UnmarshalerStream interface {
	MatchPath() string
	//UnmarshalStream is called once the patch is matched
	//the content of the message is only valid until the function return
	UnmarshalStream(key string, message json.RawMessage) error
}

// NewStreamDecoder returns a new StreamDecoder that reads from r.
//
// The StreamDecoder introduces its own buffering and may
// read data from r beyond the JSON values requested.
func NewStreamDecoder(r io.Reader) *StreamDecoder {
	return &StreamDecoder{r: r, path: newPathBuilder(), done: make(chan error, 0), context: context.Background()}
}

func (dec *StreamDecoder) WithContext(ctx context.Context) {
	dec.context = ctx
}

func (dec *StreamDecoder) Done() <-chan error { return dec.done }

func (dec *StreamDecoder) decodeBytes() ([]byte, error) {
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

	// fixup token streaming state
	dec.tokenValueEnd()

	return out, nil
}

// readValue reads a JSON value into dec.buf.
// It returns the length of the encoding.
func (dec *StreamDecoder) readValue() (int, error) {
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

func (dec *StreamDecoder) refill() error {
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
//	delim, for the four JSON delimiters [ ] { }
//	bool, for JSON booleans
//	float64, for JSON numbers
//	Number, for JSON numbers
//	string, for JSON string literals
//	nil, for JSON null
//

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
func (dec *StreamDecoder) tokenPrepareForDecode() error {
	// Note: Not calling peek before switch, to avoid
	// putting peek into the standard UnmarshalStream path.
	// peek is only called when using the Token API.
	switch dec.tokenState {
	case tokenArrayComma:
		c, err := dec.peek()
		if err != nil {
			return err
		}
		if c != ',' {
			return &SyntaxError{msg: "expected comma after array element", Offset: dec.offset()}
		}
		dec.scanp++
		dec.tokenState = tokenArrayValue
	case tokenObjectColon:
		c, err := dec.peek()
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

func (dec *StreamDecoder) tokenValueAllowed() bool {
	switch dec.tokenState {
	case tokenTopValue, tokenArrayStart, tokenArrayValue, tokenObjectValue:
		return true
	}
	return false
}

func (dec *StreamDecoder) tokenValueEnd() {
	switch dec.tokenState {
	case tokenArrayStart, tokenArrayValue:
		dec.tokenState = tokenArrayComma
	case tokenObjectValue:
		dec.tokenState = tokenObjectComma
	}
}

func (dec *StreamDecoder) Decode(itemDecoders ...UnmarshalerStream) (err error) {
	var decoders []decodeMatcher
	for i := range itemDecoders {
		matcher, err := dec.compilePath(itemDecoders[i].MatchPath())
		if err != nil {
			return err
		}
		decoders = append(decoders, decodeMatcher{decoder: itemDecoders[i], matcher: matcher})
	}
	go dec.decode(decoders...)

	select {
	case <-dec.context.Done():
		//close(dec.done)
		return dec.context.Err()
	case doneErr := <-dec.Done():
		return doneErr

	}
}

func (dec *StreamDecoder) decode(matchers ...decodeMatcher) {
	defer func() {
		close(dec.done)
	}()
	for {
		select {
		case <-dec.context.Done():
			return
		default:
		}
		c, err := dec.peek()
		if err != nil {
			if err == io.EOF {
				break
			}
			dec.done <- err
			return
		}
		switch c {
		case '[':
			if !dec.tokenValueAllowed() {
				dec.done <- dec.tokenError(c)
				return
			}
			curPath := dec.path.Path()
			match, itemDecoder := matcher(matchers).match(curPath)
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenArrayStart
			if dec.more() {
				dec.path.StartArray()
				if match {
					for {
						if !dec.more() {
							break
						}
						bytes, err := dec.decodeBytes()
						if err != nil {
							if err == io.EOF {
								break
							}
							dec.done <- err
							return
						}
						if err := itemDecoder.decoder.UnmarshalStream(curPath, bytes); err != nil {
							dec.done <- err
							return
						}
					}
				}
			}
			continue
		case ']':
			if dec.tokenState != tokenArrayStart && dec.tokenState != tokenArrayComma {
				dec.done <- dec.tokenError(c)
				return
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.path.EndObject()
			dec.tokenValueEnd()
			continue

		case '{':
			if !dec.tokenValueAllowed() {
				dec.done <- dec.tokenError(c)
				return
			}
			curPath := dec.path.Path()
			match, itemDecoder := matcher(matchers).match(curPath)
			if dec.more() {
				if match {
					if curPath == "$" {
						for {
							if !dec.more() {
								break

							}
							bytes, err := dec.decodeBytes()
							if err != nil {
								if err == io.EOF {
									break
								}
								dec.done <- err
								return
							}
							if err := itemDecoder.decoder.UnmarshalStream(curPath, bytes); err != nil {
								dec.done <- err
								return
							}
						}
					}
					bytes, err := dec.decodeBytes()
					if err != nil {
						if err == io.EOF {
							break
						}
						dec.done <- err
						return
					}

					if err := itemDecoder.decoder.UnmarshalStream(curPath, bytes); err != nil {
						dec.done <- err
						return
					}
					continue
				}
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenObjectStart
			dec.path.StartObject()
			continue

		case '}':
			if dec.tokenState != tokenObjectStart && dec.tokenState != tokenObjectComma {
				dec.done <- dec.tokenError(c)
				return
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.path.EndObject()

			dec.tokenValueEnd()
			continue
		case ':':
			if dec.tokenState != tokenObjectColon {
				dec.done <- dec.tokenError(c)
				return
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
			dec.done <- dec.tokenError(c)
			return

		case '"':
			if dec.tokenState == tokenObjectStart || dec.tokenState == tokenObjectKey {
				old := dec.tokenState
				dec.tokenState = tokenTopValue
				keyBytes, err := dec.decodeBytes()
				dec.tokenState = old
				if err != nil {
					dec.done <- err
					return
				}
				dec.tokenState = tokenObjectColon
				dec.path.SetObjectKey(keyBytes[1 : len(keyBytes)-1])
				continue
			}
			fallthrough

		default:
			if !dec.tokenValueAllowed() {
				dec.done <- dec.tokenError(c)
				return
			}

			if bytes, err := dec.decodeBytes(); err != nil {
				dec.done <- err
				return
			} else {
				curPath := dec.path.Path()
				if match, itemDecoder := matcher(matchers).match(curPath); match {
					if err := itemDecoder.decoder.UnmarshalStream(curPath, bytes); err != nil {
						dec.done <- err
						return
					}
				}
			}
		}
	}
	dec.done <- nil
	return

}

func (dec *StreamDecoder) tokenError(c byte) error {
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
	return &SyntaxError{msg: "invalid character " + quoteChar(c) + " " + context, Offset: dec.offset()}
}

// more reports whether there is another element in the
// current array or object being parsed.
func (dec *StreamDecoder) more() bool {
	c, err := dec.peek()
	return err == nil && c != ']' && c != '}'
}

func (dec *StreamDecoder) peek() (byte, error) {
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

func (dec *StreamDecoder) offset() int64 {
	return dec.scanned + int64(dec.scanp)
}

func (dec *StreamDecoder) compilePath(jsPath string) (func(curPath, jsPath string) (bool, string), error) {
	if i := strings.IndexByte(jsPath, '*'); i != -1 {
		s := escapeGlob(jsPath)
		re, err := glob.Compile(s)
		if err != nil {
			return nil, err
		}
		return func(curPath string, jsPath string) (bool, string) {
			return re.Match(curPath), curPath
		}, nil
	}
	return func(curPath string, jsPath string) (bool, string) {
		//exception for root path
		if curPath == "$" && jsPath == "$." {
			return true, jsPath
		}
		return curPath == jsPath, curPath
	}, nil
}

func escapeGlob(jsPath string) string {
	s := strings.Replace(jsPath, "[", "\\[", -1)
	s = strings.Replace(s, "]", "\\]", -1)
	return s
}

type decodeMatcher struct {
	decoder UnmarshalerStream
	matcher func(curPath, jsPath string) (bool, string)
}

type matcher []decodeMatcher

func (matchers matcher) match(curPath string) (bool, *decodeMatcher) {
	var found = -1
	for i := range matchers {
		match, _ := matchers[i].matcher(curPath, matchers[i].decoder.MatchPath())
		if match {
			if found != -1 {
				panic(matchers[found].decoder.MatchPath() + " conflicts with" + matchers[i].decoder.MatchPath())
			}
			found = i
		}
	}
	if found == -1 {
		return false, nil
	}
	return true, &matchers[found]
}

func NewRawStreamUnmarshaler(matchPath string, onMatch func(key string, message json.RawMessage) error) UnmarshalerStream {
	return &RawStreamUnmarshaler{matchPath: matchPath, onMatch: onMatch}
}

type RawStreamUnmarshaler struct {
	matchPath string
	onMatch   func(key string, message json.RawMessage) error
}

func (r *RawStreamUnmarshaler) MatchPath() string {
	return r.matchPath
}

func (r *RawStreamUnmarshaler) UnmarshalStream(key string, message json.RawMessage) error {
	return r.onMatch(key, message)
}
