package jspath

import (
	"github.com/stretchr/testify/require"
	"log"
	"testing"
)

func init() {
	log.SetFlags(log.Lshortfile)

}
func TestPath(t *testing.T) {
	path := newPathBuilder()
	path.StartObject()
	path.SetObjectKey([]byte("key"))
	path.StartArray()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	require.Equal(t, "$.key[5]", path.Path())
}

func TestPath2(t *testing.T) {
	path := newPathBuilder()
	path.StartObject()
	log.Println(path.Path())
	path.SetObjectKey([]byte("key"))
	log.Println(path.Path())
	path.StartArray()
	log.Println(path.Path())
	path.IncrementArrayIndex()
	log.Println(path.Path())
	path.IncrementArrayIndex()
	log.Println(path.Path())
	path.EndArray()
	log.Println(path.Path())
	path.EndObject()
	log.Println(path.Path())

}

func BenchmarkNewPathBuilder(b *testing.B) {
	var key = []byte("key")

	path := newPathBuilder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		path.StartObject()
		path.StartObject()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.StartArray()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.SetObjectKey(key)
		path.Path()
		path.EndObject()
		path.EndArray()
		path.Reset()

	}
}

func BenchmarkNewPathBuilder2(b *testing.B) {
	var key = []byte("key")
	path := newPathBuilder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		path.StartObject()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.StartArray()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.IncrementArrayIndex()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.Path()
		path.SetObjectKey(key)
		path.Path()
		path.EndObject()
		path.EndArray()
		path.Reset()

	}
}
