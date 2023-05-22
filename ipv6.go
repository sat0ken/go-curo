package main

import "fmt"

type ipv6Header struct {
	version      uint8
	trafficClass uint32
	headerLen    uint16
	nextHeader   uint8
	hoplimit     uint8
	srcAddr      uint64
	destAddr     uint64
}

/*
IPv6パケットの受信処理
*/
func ipv6Input(inputdev *netDevice, packet []byte) {
	// IPアドレスのついていないインターフェースからの受信は無視
	if inputdev.ipdev.address == 0 {
		return
	}
	// IPヘッダ長より短かったらドロップ
	if len(packet) < 40 {
		fmt.Printf("Received IP packet too short from %s\n", inputdev.name)
		return
	}

	ipv6hader := ipv6Header{
		version:      packet[0] >> 4,
		trafficClass: byteToUint32(packet[0:4]),
		headerLen:    byteToUint16(packet[4:6]),
		nextHeader:   packet[6],
		hoplimit:     packet[7],
		srcAddr:      byteToUint64(packet[8:24]),
		destAddr:     byteToUint64(packet[24:40]),
	}

	fmt.Printf("ipv6 packet is %+v\n", ipv6hader)
}
