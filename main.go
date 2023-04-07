package main

import (
	"flag"
)

func main() {
	var mode string
	flag.StringVar(&mode, "mode", "ch1", "set run router mode")
	flag.Parse()

	if mode == "ch1" {
		runChapter1()
	} else {
		runChapter2(mode)
	}
}
