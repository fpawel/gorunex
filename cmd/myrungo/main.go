package main

import (
	"flag"
	"github.com/fpawel/gotools/pkg/myrungo"
	"log"
)

func main() {
	log.SetFlags(log.Ltime)

	var exeName, args string
	flag.StringVar(&exeName, "exe", "", "path to executable")
	flag.StringVar(&args, "args", "", "command line arguments to pass")

	flag.Parse()

	log.Println("log file:", myrungo.LogFileName())
	if err := myrungo.Process(exeName, args, nil); err != nil {
		log.Fatal(err)
	}
}