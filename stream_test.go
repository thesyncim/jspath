package jspath

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var testdata = `
{
    "store": {
        "book": [
            {
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            },
            {
                "category": "fiction",
                "author": "Evelyn Waugh",
                "title": "Sword of Honour",
                "price": 12.99
            },
            {
                "category": "fiction",
                "author": "Herman Melville",
                "title": "Moby Dick",
                "isbn": "0-553-21311-3",
                "price": 8.99
            },
            {
                "category": "fiction",
                "author": "J. R. R. Tolkien",
                "title": "The Lord of the Rings",
                "isbn": "0-395-19395-8",
                "price": 22.99
            }
        ],
        "bicycle": {
            "color": "red",
            "price": 19.95
        }
    },
    "expensive": 10
}
`

func TestDecodePath(t *testing.T) {
	var testcases = []struct {
		name string
		path string
		want []string
	}{
		{
			name: "stream array wildcard",
			path: "$.store.book[*]",
			want: []string{
				`{
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            }`,
				`{
                "category": "fiction",
                "author": "Evelyn Waugh",
                "title": "Sword of Honour",
                "price": 12.99
            }`,
				`{
                "category": "fiction",
                "author": "Herman Melville",
                "title": "Moby Dick",
                "isbn": "0-553-21311-3",
                "price": 8.99
            }`,
				`{
                "category": "fiction",
                "author": "J. R. R. Tolkien",
                "title": "The Lord of the Rings",
                "isbn": "0-395-19395-8",
                "price": 22.99
            }`,
			},
		},
		{
			name: "index array",
			path: "$.store.book[0]",
			want: []string{
				`{
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            }`,
			},
		},
		{
			name: "index array",
			path: "$.store.book[-1]",
			want: nil,
		},
		{
			name: "array index with property",
			path: "$.store.book[0].price",
			want: []string{
				`8.95`,
			},
		},
		{
			name: "simple",
			path: "$.store.bicycle",
			want: []string{
				`{
            "color": "red",
            "price": 19.95
        }`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStreamDecoder(strings.NewReader(testdata))
			var results []json.RawMessage
			err := s.DecodePath(tc.path, func(key string, message json.RawMessage) error {
				result := make(json.RawMessage, len(message))
				copy(result, message)
				results = append(results, result)
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, len(tc.want), len(results))
			for i := range tc.want {
				require.JSONEq(t, tc.want[i], string(results[i]))
			}
		})
	}
}

var testdataMultiple = testdata + testdata

func TestDecodePathMultiple(t *testing.T) {
	var testcases = []struct {
		name string
		path string
		want []string
	}{
		{
			name: "stream array match",
			path: "$.store.book",
			want: []string{
				`[
            {
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            },
            {
                "category": "fiction",
                "author": "Evelyn Waugh",
                "title": "Sword of Honour",
                "price": 12.99
            },
            {
                "category": "fiction",
                "author": "Herman Melville",
                "title": "Moby Dick",
                "isbn": "0-553-21311-3",
                "price": 8.99
            },
            {
                "category": "fiction",
                "author": "J. R. R. Tolkien",
                "title": "The Lord of the Rings",
                "isbn": "0-395-19395-8",
                "price": 22.99
            }
        ]`,
				`[
            {
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            },
            {
                "category": "fiction",
                "author": "Evelyn Waugh",
                "title": "Sword of Honour",
                "price": 12.99
            },
            {
                "category": "fiction",
                "author": "Herman Melville",
                "title": "Moby Dick",
                "isbn": "0-553-21311-3",
                "price": 8.99
            },
            {
                "category": "fiction",
                "author": "J. R. R. Tolkien",
                "title": "The Lord of the Rings",
                "isbn": "0-395-19395-8",
                "price": 22.99
            }
        ]`,
			},
		},
		{
			name: "index array",
			path: "$.store.book[0]",
			want: []string{
				`{
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            }`, `{
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            }`},
		},
		{
			name: "index array 2",
			path: "$.store.book[0].price",
			want: []string{
				`8.95`, `8.95`,
			},
		},
		{
			name: "simple",
			path: "$.store.bicycle",
			want: []string{
				`{
            "color": "red",
            "price": 19.95
        }`,
				`{
            "color": "red",
            "price": 19.95
        }`},
		},
		{
			name: "simple",
			path: "$.store.bicycle",
			want: []string{
				`{
            "color": "red",
            "price": 19.95
        }`,
				`{
            "color": "red",
            "price": 19.95
        }`},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStreamDecoder(strings.NewReader(testdataMultiple))
			var results []json.RawMessage
			err := s.Decode(NewRawStreamUnmarshaler(tc.path, func(key string, message json.RawMessage) error {
				result := make(json.RawMessage, len(message))
				copy(result, message)
				results = append(results, result)
				return nil
			}))
			require.NoError(t, err)
			require.Equal(t, len(tc.want), len(results))
			for i := range tc.want {
				require.JSONEq(t, tc.want[i], string(results[i]))
			}
		})
	}
}

func TestDecodePathMultipleRegex(t *testing.T) {
	var testcases = []struct {
		name string
		path string
		want []string
	}{
		{
			name: "stream array match",
			path: `$.store.book[*].price`,
			want: []string{
				`8.95`,
				`12.99`,
				`8.99`,
				`22.99`,
				`8.95`,
				`12.99`,
				`8.99`,
				`22.99`,
			},
		},
		{
			name: "index array",
			path: "$.store.book[0]",
			want: []string{
				`{
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            }`, `{
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95
            }`},
		},
		{
			name: "index array 2",
			path: "$.store.book[0].price",
			want: []string{
				`8.95`, `8.95`,
			},
		},
		{
			name: "simple",
			path: "$.store.bicycle",
			want: []string{
				`{
            "color": "red",
            "price": 19.95
        }`,
				`{
            "color": "red",
            "price": 19.95
        }`},
		},
		{
			name: "simple",
			path: "$.store.bicycle",
			want: []string{
				`{
            "color": "red",
            "price": 19.95
        }`,
				`{
            "color": "red",
            "price": 19.95
        }`},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStreamDecoder(strings.NewReader(testdataMultiple))
			var results []json.RawMessage
			err := s.Decode(NewRawStreamUnmarshaler(tc.path, func(key string, message json.RawMessage) error {
				result := make(json.RawMessage, len(message))
				copy(result, message)
				results = append(results, result)
				return nil
			}))
			require.NoError(t, err)
			require.Equal(t, len(tc.want), len(results))
			for i := range tc.want {
				require.JSONEq(t, tc.want[i], string(results[i]))
			}
		})
	}
}

func TestHandleRoot(t *testing.T) {
	var testcases = []struct {
		name  string
		path  string
		input string
		want  []string
	}{
		{
			name:  "stream array match",
			path:  "$.",
			input: `["abc"]`,
			want: []string{
				`["abc"]`,
			},
		},
		{
			name:  "stream object match",
			path:  "$.",
			input: `{"abc":{}}`,
			want: []string{
				`{"abc":{}}`,
			},
		},
		{
			name:  "stream multiple object match",
			path:  "$.",
			input: `{"abc":{}}{"abc":{}}`,
			want: []string{
				`{"abc":{}}`,
				`{"abc":{}}`,
			},
		},
		{
			name:  "match stream root level strings",
			path:  "$.",
			input: `["abc"]["abc"]`,
			want: []string{
				`["abc"]`,
				`["abc"]`,
			},
		},
		{
			name:  "match stream root level strings",
			path:  "$.",
			input: `"asd" "sds"`,
			want: []string{
				`"asd"`,
				`"sds"`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStreamDecoder(strings.NewReader(tc.input))
			var results []json.RawMessage
			err := s.Decode(NewRawStreamUnmarshaler(tc.path, func(key string, message json.RawMessage) error {
				result := make(json.RawMessage, len(message))
				copy(result, message)
				results = append(results, result)
				return nil
			}))
			require.NoError(t, err)
			require.Equal(t, len(tc.want), len(results))
			for i := range tc.want {
				require.JSONEq(t, tc.want[i], string(results[i]))
			}
		})
	}
}

func TestDecodeSimpleTypesAndReset(t *testing.T) {
	var testcases = []struct {
		name  string
		path  string
		input string
		want  []string
	}{
		{
			name:  "stream array match",
			path:  "$.[0]",
			input: `["abc"]`,
			want: []string{
				`"abc"`,
			},
		},
		{
			name: "stream array match wildcard",
			path: "$.[*].abc",
			input: `[{"abc":67}]
                   [{"abc":68}]
                   [{"abc":69}]`,
			want: []string{
				`67`, "68", "69",
			},
		},
		{
			name:  "mixed root level",
			path:  "$.",
			input: `"a" 3 ["s"] {"j":"j"} false`,
			want: []string{
				`"a"`, "3", `["s"]`, `{"j":"j"}`, "false",
			},
		},
	}

	var dec *StreamDecoder
	for _, tc := range testcases {

		t.Run(tc.name, func(t *testing.T) {
			if dec == nil {
				dec = NewStreamDecoder(strings.NewReader(tc.input))
			} else {
				dec.Reset(strings.NewReader(tc.input))
			}
			var results []json.RawMessage
			err := dec.Decode(NewRawStreamUnmarshaler(tc.path, func(key string, message json.RawMessage) error {
				result := make(json.RawMessage, len(message))
				copy(result, message)
				results = append(results, result)
				return nil
			}))
			require.NoError(t, err)

			require.Equal(t, len(tc.want), len(results))
			for i := range tc.want {
				require.JSONEq(t, tc.want[i], string(results[i]))
			}
		})
	}
}

/*
func TestBigFile(t *testing.T) {
	f, err := os.Open("/Users/stream/Downloads/citylots.json")
	require.NoError(t, err)
	s := NewStreamDecoder(f)
	err = s.Decode(NewRawStreamUnmarshaler("$.features[*]", func(key string, message json.RawMessage) error {
		return nil
	}))
	require.NoError(t, err)
}*/
