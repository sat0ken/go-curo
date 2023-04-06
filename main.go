package main

import (
	"flag"
	"fmt"
)

func main() {
	var mode string
	flag.StringVar(&mode, "mode", "ch1", "set run router mode")
	flag.Parse()

	if mode == "ch1" {
		runChapter1()
	} else if mode == "ch5" {
		runChapter2(mode)
	} else {
		calcsum()
	}
}

func calcsum() {
	var checksum uint32
	dummy := dummyHeader{
		srcAddr:  byteToUint32([]byte{0xc0, 0xa8, 0x02, 0x02}),
		destAddr: byteToUint32([]byte{0xc0, 0xa8, 0x01, 0x03}),
		protocol: 0x06,
		length:   40,
	}
	tcp := tcpHeader{
		srcPort:    11111,
		destPort:   20000,
		seq:        1587642247,
		ackseq:     800028691,
		offset:     160,
		tcpflag:    18,
		window:     65160,
		checksum:   0,
		urgPointer: 0,
		// port 20000
		//options: []byte{
		//	0x02, 0x04, 0x05, 0xb4, 0x04, 0x02, 0x08, 0x0a,
		//	0x7e, 0x5b, 0x37, 0xf5, 0x46, 0x1f, 0x78, 0x44,
		//	0x01, 0x03, 0x03, 0x07,
		//},

		options: []byte{
			0x02, 0x04, 0x05, 0xb4, 0x04, 0x02, 0x08, 0x0a,
			0x83, 0x93, 0xe7, 0x35, 0x4b, 0x58, 0x2b, 0x84,
			0x01, 0x03, 0x03, 0x07,
		},
	}

	packet := dummy.ToPacket()
	packet = append(packet, tcp.ToPacket()...)

	fmt.Printf("checksum is %x\n", calcChecksum(packet))

	checksum = uint32(byteToUint16(calcChecksum(packet)))
	fmt.Printf("checksum is %d\n", checksum^0xffff)
	checksum = checksum ^ 0xffff

	// dest
	checksum += (byteToUint32([]byte{0xc0, 0xa8, 0x01, 0x03}) - byteToUint32([]byte{0xc0, 0xa8, 0x00, 0x01}))
	// dest port
	checksum += 30000 - 20000
	//dummy.destAddr = byteToUint32([]byte{0xc0, 0xa8, 0x01, 0x03})
	//tcp.destPort = 30000
	//packet = []byte{}
	//packet = dummy.ToPacket()
	//packet = append(packet, tcp.ToPacket()...)

	fmt.Printf("ip addr diff is %d\n", byteToUint32([]byte{0xc0, 0xa8, 0x01, 0x03})-byteToUint32([]byte{0xc0, 0xa8, 0x00, 0x01}))

	fmt.Printf("re calc checksum is %d\n", checksum)

	checksum = (0x74b7 & 0xffff) + 0x74b7>>16
	fmt.Printf("checksum is %x\n", checksum^0xffff)

	fmt.Printf("bad is %d, good is %d\n", 0xe4fa^0xffff, 0xe3f8^0xffff)

}
