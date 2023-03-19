package main

import "fmt"

func main() {
	// synパケット
	sum := 0xf903
	ocsum := sum ^ 0xffff

	// destのIPアドレスを引く
	ocsum -= (0xc0a80001 - 0xc0a80103)

	fmt.Printf("ocsum^0xffff is %x\n", ocsum^0xffff)
}

func syn() {
	// synパケット
	sum := 0x6a93
	ocsum := sum ^ 0xffff

	// sourceのIPアドレスを引く
	ocsum -= (0xc0a80103 - 0xc0a80001)

	fmt.Printf("ocsum^0xffff is %x\n", ocsum^0xffff)
}
