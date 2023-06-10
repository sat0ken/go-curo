package main

import (
	"bytes"
	"fmt"
)

const (
	// https://tex2e.github.io/rfc-translater/html/rfc4443.html
	ICMPv6_TYPE_ECHO_REQUEST           uint8 = 128
	ICMPv6_TYPE_ECHO_REPLY             uint8 = 129
	ICMPv6_TYPE_Neighbor_Solicitation  uint8 = 135
	ICMPv6_TYPE_Neighbor_Advertisement uint8 = 136
)

/*
https://tex2e.github.io/rfc-translater/html/rfc4443.html
2.1. Message General Format

	 0                   1                   2                   3
	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|     Type      |     Code      |          Checksum             |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|                                                               |
	+                         Message Body                          +
	|                                                               |
*/
type icmpv6Message struct {
	icmpType uint8
	icmpCode uint8
	checksum uint16
	message  any
}

// https://tex2e.github.io/rfc-translater/html/rfc4861.html#4-3--Neighbor-Solicitation-Message-Format
type icmpv6NeighborSolicitation struct {
	reserved   uint32
	targetAddr [16]byte
	optnd      optNeighborDiscovery
}

// https://tex2e.github.io/rfc-translater/html/rfc4861.html#4-4--Neighbor-Advertisement-Message-Format
type icmpv6NeighborAdvertisement struct {
	flagRouter    bool
	flagSolicited bool
	flagOverRide  bool
	reserved      uint32 // Reserved 29-bit unused field
	targetAddr    [16]byte
	optnd         optNeighborDiscovery
}

// https://tex2e.github.io/rfc-translater/html/rfc4861.html#4-6--Option-Formats
type optNeighborDiscovery struct {
	opttype uint8
	length  uint8
	macaddr [6]uint8
}

func (na *icmpv6NeighborAdvertisement) ToPacket() []byte {
	var b bytes.Buffer
	flagsbytes := uint32ToByte(na.reserved)
	if na.flagRouter {
		flagsbytes[0] += 128
	}
	if na.flagSolicited {
		flagsbytes[0] += 64
	}
	if na.flagOverRide {
		flagsbytes[0] += 32
	}
	b.Write(flagsbytes)
	b.Write(na.targetAddr[:])
	// option
	b.Write([]byte{na.optnd.opttype, na.optnd.length})
	b.Write(macToByte(na.optnd.macaddr))

	return b.Bytes()
}

func (icmpmsg *icmpv6Message) Replyv6Packet(sourceAddr, destAddr [16]byte) (icmpv6Packet []byte) {
	var b bytes.Buffer
	// ICMPv6ヘッダ
	b.Write([]byte{ICMPv6_TYPE_ECHO_REPLY})
	b.Write([]byte{0x00})       // icmp code
	b.Write([]byte{0x00, 0x00}) // checksum
	// ICMPエコーメッセージ
	echomessage := icmpmsg.message.(icmpEcho)
	b.Write(uint16ToByte(echomessage.identify))
	b.Write(uint16ToByte(echomessage.sequence))
	b.Write(echomessage.data)

	// いったんパケットデータにする
	icmpv6Packet = b.Bytes()
	// IPv6ダミーヘッダをセット
	dumyv6Header := ipv6DummyHeader{
		srcAddr:  sourceAddr,
		destAddr: destAddr,
		length:   uint32(len(icmpv6Packet)),
		protocol: uint32(IP_PROTOCOL_NUM_ICMPv6),
	}
	// チェックサム計算用のデータを生成
	calcPacket := dumyv6Header.ToPacket()
	calcPacket = append(calcPacket, icmpv6Packet...)

	// チェックサムを計算
	checksum := calcChecksum(calcPacket)

	// 計算したチェックサムをセット
	icmpv6Packet[2] = checksum[0]
	icmpv6Packet[3] = checksum[1]

	return icmpv6Packet
}

func (icmpmsg *icmpv6Message) ReplyNeighborAdvertisement(sourceAddr, destAddr [16]byte, sourceMacAddr [6]uint8) (icmpv6Packet []byte) {
	var b bytes.Buffer
	// ICMPv6ヘッダ
	b.Write([]byte{ICMPv6_TYPE_Neighbor_Advertisement})
	b.Write([]byte{0x00})       // icmp code
	b.Write([]byte{0x00, 0x00}) // checksum
	// Neighbor Advertisementメッセージ
	na := icmpv6NeighborAdvertisement{
		flagRouter:    false,
		flagSolicited: true,
		flagOverRide:  true,
		reserved:      0,
		targetAddr:    icmpmsg.message.(icmpv6NeighborSolicitation).targetAddr,
		optnd: optNeighborDiscovery{
			opttype: 2,
			length:  1,
			macaddr: sourceMacAddr,
		},
	}
	b.Write(na.ToPacket())
	// いったんパケットデータにする
	icmpv6Packet = b.Bytes()
	// IPv6ダミーヘッダをセット
	dumyv6Header := ipv6DummyHeader{
		srcAddr:  sourceAddr,
		destAddr: destAddr,
		length:   uint32(len(icmpv6Packet)),
		protocol: uint32(IP_PROTOCOL_NUM_ICMPv6),
	}
	// チェックサム計算用のデータを生成
	calcPacket := dumyv6Header.ToPacket()
	calcPacket = append(calcPacket, icmpv6Packet...)

	// チェックサムを計算
	checksum := calcChecksum(calcPacket)

	// 計算したチェックサムをセット
	icmpv6Packet[2] = checksum[0]
	icmpv6Packet[3] = checksum[1]

	return icmpv6Packet
}

func icmpv6Input(inputdev *netDevice, sourceAddr, destAddr [16]byte, icmpPacket []byte) {
	// ICMPメッセージ長より短かったら
	if len(icmpPacket) < 4 {
		fmt.Println("Received ICMPv6 Packet is too short")
	}
	// ICMPv6のヘッダ部を解釈する
	icmpmsg := icmpv6Message{
		icmpType: icmpPacket[0],
		icmpCode: icmpPacket[1],
		checksum: byteToUint16(icmpPacket[2:4]),
	}

	fmt.Printf("receive icmpv6 packet, type is %d\n", icmpmsg.icmpType)

	switch icmpmsg.icmpType {
	case ICMPv6_TYPE_ECHO_REPLY:
		fmt.Println("ICMPv6 ECHO REPLY is received")
	case ICMPv6_TYPE_ECHO_REQUEST:
		fmt.Printf("ICMPv6 ECHO REQUEST is received, Create Reply Packet to %x\n", destAddr)
		icmpmsg.message = icmpEcho{
			identify: byteToUint16(icmpPacket[4:6]),
			sequence: byteToUint16(icmpPacket[6:8]),
			data:     icmpPacket[8:],
		}
		payload := icmpmsg.Replyv6Packet(sourceAddr, destAddr)
		ipv6PacketEncapsulateOutput(inputdev, sourceAddr, destAddr, payload, IP_PROTOCOL_NUM_ICMPv6)
	case ICMPv6_TYPE_Neighbor_Solicitation:
		fmt.Println("ICMPv6 Neighbor_Solicitation is received")
		icmpmsg.message = icmpv6NeighborSolicitation{
			reserved:   byteToUint32(icmpPacket[4:8]),
			targetAddr: setipv6addr(icmpPacket[8:24]),
			optnd: optNeighborDiscovery{
				opttype: icmpPacket[25],
				length:  icmpPacket[26],
				macaddr: setMacAddr(icmpPacket[26:32]),
			},
		}
		payload := icmpmsg.ReplyNeighborAdvertisement(sourceAddr, destAddr, inputdev.macaddr)
		ipv6PacketEncapsulateOutput(inputdev, sourceAddr, destAddr, payload, IP_PROTOCOL_NUM_ICMPv6)
	}
}
