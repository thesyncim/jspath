package main

import (
	"encoding/json"
	"github.com/thesyncim/gojq"
	"log"
	"os"

	"github.com/urfave/cli"
)

var (
	src      string
	printKey bool
)

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
			return gojq.NewDecoder(f).DecodePathKey(jsPath, func(key string, message json.RawMessage) error {
				_, err := os.Stdout.WriteString(key)
				_, err = os.Stdout.WriteString("=>")
				_, err = os.Stdout.Write(message)
				_, err = os.Stdout.Write([]byte("\n"))
				return err
			})
		}
		return gojq.NewDecoder(f).DecodePath(jsPath, func(message json.RawMessage) error {
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

var newLine = []byte("\n")
