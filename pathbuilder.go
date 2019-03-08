package jspath

import (
	"reflect"
	"runtime"
	"strconv"
	"unsafe"
)

func BytesToString(bytes []byte) (s string) {
	slice := (*reflect.SliceHeader)(unsafe.Pointer(&bytes))
	str := (*reflect.StringHeader)(unsafe.Pointer(&s))
	str.Data = slice.Data
	str.Len = slice.Len
	runtime.KeepAlive(&bytes) // this line is essential.
	return s
}

type pathBuilder struct {
	path               []byte
	stackSegmentsSizes sizeStacks

	indexBuf [4 * 16]byte
}

func newPathBuilder() pathBuilder {
	p := pathBuilder{
		path:               make([]byte, 0, 256),
		stackSegmentsSizes: make([]int, 0, 256),
	}
	p.Reset()
	return p
}

func (pb *pathBuilder) Reset() {
	pb.path = pb.path[0:0]
	pb.stackSegmentsSizes = pb.stackSegmentsSizes[0:0]
	pb.extend(1)
	pb.path[len(pb.path)-1] = '$'
	pb.stackSegmentsSizes.Push(1)
}

func (pb *pathBuilder) StartObject() {
	pb.stackSegmentsSizes.Push(0)
}

func (pb *pathBuilder) EndObject() {
	pb.shrink(pb.stackSegmentsSizes.Pop())
}

func (pb *pathBuilder) StartArray() {
	//its a noop
	if len(pb.path) < 2 {
		pb.extend(4)
		pathLen := len(pb.path)
		pb.path[pathLen-4] = '.'
		pb.path[pathLen-3] = '['
		pb.path[pathLen-2] = '0'
		pb.path[pathLen-1] = ']'
		pb.stackSegmentsSizes.Push(4)
		return
	}
	pb.extend(3)
	pathLen := len(pb.path)
	pb.path[pathLen-3] = '['
	pb.path[pathLen-2] = '0'
	pb.path[pathLen-1] = ']'
	pb.stackSegmentsSizes.Push(3)
}

func (pb *pathBuilder) EndArray() {
	pb.shrink(pb.stackSegmentsSizes.Pop())
}

func (pb *pathBuilder) IncrementArrayIndex() {
	size := pb.stackSegmentsSizes.Pop()
	array := pb.path[len(pb.path)-size:]
	start := 1
	if array[0] == '.' {
		start = 2
	}
	index := array[start : len(array)-1]

	i, err := strconv.Atoi(BytesToString(index))
	if err != nil {
		panic(err)
	}
	i++
	incremented := strconv.AppendInt(pb.indexBuf[:0], int64(i), 10)
	newSize := len(incremented) + 2
	if start > 1 {
		newSize++
	}
	pb.stackSegmentsSizes.Push(newSize)
	pb.shrink(size)
	pb.extend(newSize)
	copy(pb.path[len(pb.path)-newSize+start:], incremented)
	pb.path[len(pb.path)-1] = ']'
}

func (pb *pathBuilder) SetObjectKey(key []byte) {
	pb.shrink(pb.stackSegmentsSizes.Pop())
	dotPlusKeySize := 1 + len(key)
	pb.extend(dotPlusKeySize)
	pb.path[len(pb.path)-dotPlusKeySize] = '.'
	copy(pb.path[len(pb.path)-dotPlusKeySize+1:], key)
	pb.stackSegmentsSizes.Push(dotPlusKeySize)
}

func (pb *pathBuilder) Path() string {
	return *(*string)(unsafe.Pointer(&pb.path))
}

func (pb *pathBuilder) PathBytes() []byte {
	return pb.path
}

func (pb *pathBuilder) extend(n int) {
	newSize := len(pb.path) + n
	if newSize >= cap(pb.path) {
		panic("please increase math path length")
	}
	pb.path = pb.path[:newSize]
}

func (pb *pathBuilder) shrink(n int) {
	if n == 0 {
		return
	}
	newSize := len(pb.path) - n
	if newSize > 0 {
		pb.path = pb.path[:newSize]
		return
	}
}

type sizeStacks []int

func (stack *sizeStacks) Len() int {
	return len(*stack)
}

func (stack *sizeStacks) Peek() int {
	return (*stack)[len(*stack)-1]
}

func (stack *sizeStacks) Pop() int {
	v := stack.Peek()
	*stack = (*stack)[:len(*stack)-1]
	return v
}

func (stack *sizeStacks) Push(v int) {
	*stack = (*stack)[:len(*stack)+1]
	(*stack)[len(*stack)-1] = v
}
