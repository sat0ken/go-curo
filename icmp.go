package main

import "fmt"

const (
	ICMP_TYPE_ECHO_REPLY              uint8 = 0
	ICMP_TYPE_DESTINATION_UNREACHABLE uint8 = 3
	ICMP_TYPE_ECHO_REQUEST            uint8 = 8
	ICMP_TYPE_TIME_EXCEEDED           uint8 = 11
)

type icmpHeader struct {
	icmpType uint8
	icmpCode uint8
	checksum uint16
}

type icmpEcho struct {
	identify  uint16
	sequence  uint16
	timestamp []uint8
	data      []uint8
}

type icmpDestinationUnreachable struct {
	unused uint32
	data   []uint8
}

type icmpTimeExceeded struct {
	unused uint32
	data   []uint8
}

type icmpMessage struct {
	icmpHeader                 icmpHeader
	icmpEcho                   icmpEcho
	icmpDestinationUnreachable icmpDestinationUnreachable
	icmpTimeExceeded           icmpTimeExceeded
}

func icmpInput(sourceAddr, destAddr uint32, icmpPacket []byte) {
	// ICMPメッセージ長より短かったら
	if len(icmpPacket) < 4 {
		fmt.Println("Received ICMP Packet is too short")
	}
	// ICMPのパケットとして解釈する
	icmpmsg := icmpMessage{
		icmpHeader: icmpHeader{
			icmpType: icmpPacket[0],
			icmpCode: icmpPacket[1],
			checksum: byteToUint16(icmpPacket[2:4]),
		},
		icmpEcho: icmpEcho{
			identify:  byteToUint16(icmpPacket[4:6]),
			sequence:  byteToUint16(icmpPacket[6:8]),
			timestamp: icmpPacket[8:16],
			data:      icmpPacket[16:],
		},
	}
	fmt.Printf("ICMP Packet is %+v\n", icmpmsg)

	switch icmpmsg.icmpHeader.icmpType {
	case ICMP_TYPE_ECHO_REPLY:
		fmt.Println("ICMP ECHO REPLY is received")
	case ICMP_TYPE_ECHO_REQUEST:
		// Todo: ICMPリプライを作成
	}
}
