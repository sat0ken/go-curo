package main

import "fmt"

func main() {
	sum := 0xb769
	ocsum := sum ^ 0xffff

	ocsum -= (0xc0a80001 - 0xc0a80103)
	// checksum
	ocsum -= (0x4006 - 0x3f06)

	fmt.Printf("ocsum^0xffff is %x\n", ocsum^0xffff)
}

func _() {
	// sum := 0x39a3
	sum := 0xc85e
	ocsum := sum ^ 0xffff

	// sourceのIPアドレスを引く
	ocsum -= (0xc0a80103 - 0xc0a80001)
	// checksum
	ocsum -= (0x4006 - 0x3f06)

	// ルータのIPアドレスを引く
	//ocsum += 0x0003
	//ocsum += 0x3f06

	//ip2 := ip1 - (0xffff ^ 0x0103) - 0x0003
	// sum = (sum & 0xffff) + sum>>16

	fmt.Printf("ocsum is %x\n", ocsum)
	fmt.Printf("ocsum is %d\n", ocsum)

	fmt.Printf("ocsum^0xffff is %x\n", ocsum^0xffff)

	//fmt.Printf("ip diff is %d\n", 0xc0a80103-0xc0a80003)
	//fmt.Printf("checksum diff is %d\n", 0x40-0x3f)

	//fmt.Printf("sum is %16b\n", sum)
	//fmt.Printf("sum is %16b\n", 0xffff)
}

func _() {
	hc := (0xCD7A + 0x5555)
	// ヘッダの1の補数和
	hc = (hc & 0xffff) + hc>>16 // 22d0

	// 論理否定を取った値=元のchecksum
	hc = hc ^ 0xffff // dd2f

	fmt.Printf("hc is %x\n", hc)

	hc = hc - (0xffff ^ 0x5555) - 0x3285
	fmt.Printf("hc is %x\n", hc)
}
