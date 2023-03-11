package main

import "fmt"

//func main() {
//	var mode string
//	flag.StringVar(&mode, "mode", "ch1", "set run router mode")
//	flag.Parse()
//
//	if mode == "ch1" {
//		runChapter1()
//	} else {
//		runChapter2(mode)
//	}
//}

func main() {
	packet_host1 := []byte{
		0x45, 0x00, 0x00, 0x3c, 0xf0, 0x0a, 0x40, 0x00,
		0x40, 0x06, 0xc6, 0x5c, 0xc0, 0xa8, 0x01, 0x03,
		0xc0, 0xa8, 0x02, 0x01,
	}
	ipheader := ipHeader{
		version:        packet_host1[0] >> 4,
		headerLen:      packet_host1[0] << 5 >> 5,
		tos:            packet_host1[1],
		totalLen:       byteToUint16(packet_host1[2:4]),
		identify:       byteToUint16(packet_host1[4:6]),
		fragOffset:     byteToUint16(packet_host1[6:8]),
		ttl:            packet_host1[8],
		protocol:       packet_host1[9],
		headerChecksum: byteToUint16(packet_host1[10:12]),
		srcAddr:        byteToUint32(packet_host1[12:16]),
		destAddr:       byteToUint32(packet_host1[16:20]),
	}

	fmt.Printf("packet is %x\n", packet_host1)

	ipheader.ttl -= 1
	ipheader.srcAddr = 0xc0a80001
	ipheader.headerChecksum = 0

	fmt.Printf("packet is %x\n", ipheader.ToPacket())
}
