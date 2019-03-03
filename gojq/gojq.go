package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/thesyncim/jspath"
	"github.com/urfave/cli"
)

var (
	src      string
	printKey bool
)

var newLine = []byte("\n")

func main() {
	app := cli.NewApp()

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "key",
			Destination: &printKey,
			Usage:       "print the key along with the value",
		},
		cli.StringFlag{
			Name:        "src",
			Destination: &src,
			Usage:       "input file",
		},
	}

	app.Action = func(c *cli.Context) error {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return err
		}
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		jsPath := c.Args().Get(0)
		if printKey {
			streamChan := &ItemStreamer{items: make(chan json.RawMessage, 0), path: jsPath}
			dec := jspath.NewDecoder(f)
			go dec.DecodeStreamItems(streamChan)
			for {
				select {
				case v, ok := <-streamChan.items:
					if !ok {
						panic(ok)
					}
					os.Stdout.Write(v)
					os.Stdout.Write(newLine)
				case err := <-dec.Done():
					if err != nil {
						panic(err)
					}
					return nil
				}
			}
		}
		return jspath.NewDecoder(f).DecodeStream(jsPath, func(message json.RawMessage) error {
			_, err := os.Stdout.Write(message)
			_, err = os.Stdout.Write(newLine)
			return err
		})
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

type ItemStreamer struct {
	items chan json.RawMessage
	path  string
}

func (c *ItemStreamer) Path() string {
	return c.path
}

func (c *ItemStreamer) UnmarshalStream(key string, item json.RawMessage) error {
	c.items <- item
	return nil
}
