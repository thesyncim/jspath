package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/thesyncim/jspath"
)

var sample = `
{ "store": {
    "book": [ 
      { "category": "reference",
        "author": "Nigel Rees",
        "title": "Sayings of the Century",
        "price": 8.95
      },
      { "category": "fiction",
        "author": "Evelyn Waugh",
        "title": "Sword of Honour",
        "price": 12.99
      },
      { "category": "fiction",
        "author": "Herman Melville",
        "title": "Moby Dick",
        "isbn": "0-553-21311-3",
        "price": 8.99
      },
      { "category": "fiction",
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
  }
}
`

type Book struct {
	Category string  `json:"category"`
	Author   string  `json:"author"`
	Title    string  `json:"title"`
	Price    float64 `json:"price"`
	Isbn     string  `json:"isbn,omitempty"`
}

type Bicycle struct {
	Color string  `json:"color"`
	Price float64 `json:"price"`
}

type BookStreamer chan *Book

func (BookStreamer) AtPath() string {
	return "$.store.book[*]"
}

func (bs BookStreamer) UnmarshalStream(key string, message json.RawMessage) error {
	var b Book
	if err := json.Unmarshal(message, &b); err != nil {
		return err
	}
	bs <- &b
	return nil
}

type BicycleStreamer chan *Bicycle

func (BicycleStreamer) AtPath() string {
	return "$.store.bicycle"
}

func (bs BicycleStreamer) UnmarshalStream(key string, message json.RawMessage) error {
	var b Bicycle
	if err := json.Unmarshal(message, &b); err != nil {
		return err
	}
	bs <- &b
	return nil
}

func main() {
	repeat := 100
	payload := strings.Repeat(sample, repeat)

	decoder := jspath.NewStreamDecoder(strings.NewReader(payload))

	bookStreamer := BookStreamer(make(chan *Book))
	bicycleStreamer := BicycleStreamer(make(chan *Bicycle))

	go decoder.Decode(bookStreamer, bicycleStreamer)

	var totalBicycles int
	var totalBooks int
	var done bool
	for {
		select {
		case err := <-decoder.Done():
			done = true
			if err != nil {
				panic(err)
			}
			break
		case book := <-bookStreamer:
			totalBooks++
			//handle book
			log.Println(book)
		case bicycle := <-bicycleStreamer:
			totalBicycles++
			//handle bicycle
			log.Println(bicycle)
		}
		if done {
			break
		}
	}
	if repeat*4 != totalBooks {
		panic(fmt.Sprintf("expecting %d books and got %d", repeat*4, totalBooks))
	}
	if repeat != totalBicycles {
		panic(fmt.Sprintf("expecting %d bicycles and got %d", repeat, totalBicycles))
	}
}
