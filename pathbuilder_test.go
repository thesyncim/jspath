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
	path.StartArray()
	path = newPathBuilder()
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

func BenchmarkNewPathBuilder(b *testing.B) {

	path := newPathBuilder()
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
