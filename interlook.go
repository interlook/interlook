package main

import (
	"flag"
	"github.com/interlook/interlook/core"
)

//go:generate go run gen.go

var configFile string

func main() {
	if flag.Lookup("conf") == nil {
		flag.StringVar(&configFile, "conf", "", "interlook configuration file")
	}
	flag.Parse()
	core.Start(configFile)
}
