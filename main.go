package main

import (
	"flag"
)

func main() {
	var chapter1 bool
	var chapter2 bool
	flag.BoolVar(&chapter1, "chapter1", false, "run chapter1")
	flag.BoolVar(&chapter2, "chapter2", false, "run chapter2")
	flag.Parse()

	if chapter1 {
		runChapter1()
	} else if chapter2 {
		runChapter2()
	}

}
