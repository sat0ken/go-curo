package main

import (
	"bytes"
	"fmt"
)

const (
	// https://tex2e.github.io/rfc-translater/html/rfc4443.html
	ICMPv6_TYPE_ECHO_REQUEST uint8 = 128
	ICMPv6_TYPE_ECHO_REPLY   uint8 = 129
)

func (icmpmsg icmpMessage) Replyv6Packet(sourceAddr, destAddr [16]byte) (icmpv6Packet []byte) {
	var b bytes.Buffer
	// ICMPv6ヘッダ
	b.Write([]byte{ICMPv6_TYPE_ECHO_REPLY})
	b.Write([]byte{0x00})       // icmp code
	b.Write([]byte{0x00, 0x00}) // checksum
	// ICMPエコーメッセージ
	b.Write(uint16ToByte(icmpmsg.icmpEcho.identify))
	b.Write(uint16ToByte(icmpmsg.icmpEcho.sequence))
	b.Write(icmpmsg.icmpEcho.data)

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
	icmpmsg := icmpMessage{
		icmpHeader: icmpHeader{
			icmpType: icmpPacket[0],
			icmpCode: icmpPacket[1],
			checksum: byteToUint16(icmpPacket[2:4]),
		},
	}

	switch icmpmsg.icmpHeader.icmpType {
	case ICMPv6_TYPE_ECHO_REPLY:
		fmt.Println("ICMPv6 ECHO REPLY is received")
	case ICMPv6_TYPE_ECHO_REQUEST:
		fmt.Println("ICMPv6 ECHO REQUEST is received, Create Reply Packet")
		icmpmsg.icmpEcho = icmpEcho{
			identify: byteToUint16(icmpPacket[4:6]),
			sequence: byteToUint16(icmpPacket[6:8]),
			data:     icmpPacket[8:],
		}
		payload := icmpmsg.Replyv6Packet(sourceAddr, destAddr)
		ipv6PacketEncapsulateOutput(inputdev, sourceAddr, destAddr, payload, IP_PROTOCOL_NUM_ICMPv6)
	}
}
