package gojq

import (
	"bytes"
	"reflect"
	"runtime"
	"strconv"
	"unsafe"
)

type PathBuilder struct {
	Segments []bytes.Buffer

	maxSegments int

	pathBuf  bytes.Buffer
	indexBuf [8]byte
}

func NewPathBuilder() *PathBuilder {
	p := PathBuilder{maxSegments: 200}
	p.init()
	p.Extend()
	p.LastSegment().WriteByte('$')
	return &p
}

func (p *PathBuilder) init() {
	p.Segments = make([]bytes.Buffer, 0, p.maxSegments)
}

func (p *PathBuilder) StartObject() {
	p.Extend()
}

func (p *PathBuilder) SetObjectKey(key []byte) {
	lastSegment := p.LastSegment()
	lastSegment.Reset()
	lastSegment.WriteByte('.')
	lastSegment.Write(key)
}

func (p *PathBuilder) EndObject() {
	p.RemoveLastSegment()
}

func (p *PathBuilder) EndArray() {
	p.RemoveLastSegment()
}

func (p *PathBuilder) StartArray() {
	if len(p.Segments) < 2 {
		p.Extend()
		p.LastSegment().WriteByte('.')
	}
	p.Extend()
	p.LastSegment().WriteString("[0]")
}

func (p *PathBuilder) IncrementArrayIndex() {
	index := p.LastSegment().Bytes()[1 : len(p.LastSegment().Bytes())-1]
	i, err := strconv.Atoi(BytesToString(index))
	if err != nil {
		panic(err)
	}
	i++
	incremented := strconv.AppendInt(p.indexBuf[:0], int64(i), 10)
	p.LastSegment().Reset()
	p.LastSegment().WriteByte('[')
	p.LastSegment().Write(incremented)
	p.LastSegment().WriteByte(']')

}
func (p *PathBuilder) LastSegment() *bytes.Buffer {
	return &p.Segments[len(p.Segments)-1]
}

func (p *PathBuilder) Extend() {
	p.Segments = p.Segments[:len(p.Segments)+1]
}

func (p *PathBuilder) RemoveLastSegment() {
	p.LastSegment().Reset()
	p.Segments = p.Segments[:len(p.Segments)-1]
}

func (p *PathBuilder) Path() string {
	p.pathBuf.Reset()
	for i := range p.Segments {
		if p.Segments[i].Len() != 0 {
			p.pathBuf.Write(p.Segments[i].Bytes())
		}
	}
	s := p.pathBuf.Bytes()
	return *(*string)(unsafe.Pointer(&s))
}

func (p *PathBuilder) MatchString(jsPath string) bool {
	return p.Path() == jsPath
}

func BytesToString(bytes []byte) (s string) {
	slice := (*reflect.SliceHeader)(unsafe.Pointer(&bytes))
	str := (*reflect.StringHeader)(unsafe.Pointer(&s))
	str.Data = slice.Data
	str.Len = slice.Len
	runtime.KeepAlive(&bytes) // this line is essential.
	return s
}
