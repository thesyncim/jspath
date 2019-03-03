package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/pkg/profile"
	"github.com/thesyncim/jspath"
	"github.com/urfave/cli"
)

var (
	src      string
	printKey bool
	debug    bool
	out      string
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
		cli.BoolFlag{
			Name:        "debug",
			Destination: &debug,
			Usage:       "debug print memstats",
		},
		cli.StringFlag{
			Name:        "src",
			Destination: &src,
			Usage:       "input file",
		},
		cli.StringFlag{
			Name:        "o",
			Destination: &out,
			Usage:       "output file default stdout",
		},
	}

	app.Action = func(c *cli.Context) error {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return err
		}
		var outWriter io.Writer
		var err error
		outWriter = os.Stdout
		if out != "" {
			outWriter, err = os.OpenFile(out, os.O_APPEND|os.O_CREATE, 0644)
			if err != nil {
				return err
			}
			outWriter = bufio.NewWriter(outWriter)
		}
		source, err := os.Open(src)
		if err != nil {
			return err
		}

		if debug {
			p := startDebug()
			defer p.Stop()
		}

		defer source.Close()
		var pathStreamers []jspath.UnmarshalerStream
		for _, path := range c.Args() {
			pathStreamers = append(pathStreamers, jspath.NewRawStreamUnmarshaler(path, func(key []byte, message json.RawMessage) error {
				if printKey {
					outWriter.Write(key)
					outWriter.Write(space)
				}
				outWriter.Write(message)
				outWriter.Write(newLine)
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
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func startDebug() interface{ Stop() } {
	p := profile.Start(profile.CPUProfile, profile.ProfilePath("."), profile.NoShutdownHook)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		go func() {
			var m runtime.MemStats
			for {
				runtime.ReadMemStats(&m)
				// For info on each, see: https://golang.org/pkg/runtime/#MemStats
				fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
				fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
				fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
				fmt.Printf("\tNumGC = %v\n", m.NumGC)
				time.Sleep(time.Second)
			}
		}()
		for sig := range ch {
			log.Printf("captured %v, stopping profiler and exiting..", sig)
			p.Stop()
			os.Exit(1)
		}
	}()
	return p
}
