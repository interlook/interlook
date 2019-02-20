package main

import (
	"log"

	"github.com/bhuisgen/interlook/core"
)

var (
	Version = "0.1.0"
)

func main() {
	log.Println("[INFO]", "interlookd", Version)
	core.Start()
}
