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
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		jsPath := c.Args().Get(0)
		return jspath.NewStreamDecoder(f).Decode(jspath.NewRawStreamUnmarshaler(jsPath, func(key string, message json.RawMessage) error {
			if printKey {
				os.Stdout.WriteString(key)
				os.Stdout.Write(space)
			}
			_, err := os.Stdout.Write(message)
			_, err = os.Stdout.Write(newLine)
			return err
		}))
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
