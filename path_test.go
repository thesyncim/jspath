package gojq

import (
	"github.com/gobwas/glob"
	"log"
	"testing"
)

func TestPath(t *testing.T) {
	path := NewPathBuilder()
	path.StartArray()
	log.Println(path.Path())

	path = NewPathBuilder()
	path.StartObject()
	path.SetObjectKey([]byte("key"))
	path.StartArray()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	path.IncrementArrayIndex()
	log.Println(path.Path())
}

func BenchmarkNewPathBuilder(b *testing.B) {

	path := NewPathBuilder()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		path.StartObject()
		path.StartObject()

		path.SetObjectKey([]byte("key"))
		path.Path()
		path.EndObject()
		path.EndArray()

	}
}

func TestPath_MatchString(t *testing.T) {
	exp := glob.MustCompile(`$.abc\[*\]`)
	log.Println(exp.Match("$.abc[0]"))
}
