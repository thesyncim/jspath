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
var space = []byte(" ")

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
		source, err := os.Open(src)
		if err != nil {
			return err
		}
		defer source.Close()
		var pathStreamers []jspath.UnmarshalerStream
		for _, path := range c.Args() {
			pathStreamers = append(pathStreamers, jspath.NewRawStreamUnmarshaler(path, func(key string, message json.RawMessage) error {
				if printKey {
					os.Stdout.WriteString(key)
					os.Stdout.Write(space)
				}
				os.Stdout.Write(message)
				os.Stdout.Write(newLine)
				return nil
			}))
		}
		return jspath.NewStreamDecoder(source).Decode(pathStreamers...)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
