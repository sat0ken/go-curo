package main

import (
	"bytes"
	"fmt"
)

type ipv6Header struct {
	version      uint8
	trafficClass uint8
	flowLabel    uint32
	headerLen    uint16
	nextHeader   uint8
	hoplimit     uint8
	srcAddr      uint64
	destAddr     uint64
}

// チェックサム計算用の疑似ヘッダ
type ipv6DummyHeader struct {
	srcAddr  uint64
	destAddr uint64
	length   uint32
	protocol uint32
}

func (ipv6Dummy *ipv6DummyHeader) ToPacket() []byte {
	var b bytes.Buffer

	b.Write(uint64ToByte(ipv6Dummy.srcAddr))
	b.Write(uint64ToByte(ipv6Dummy.destAddr))
	b.Write(uint32ToByte(ipv6Dummy.length))
	b.Write(uint32ToByte(ipv6Dummy.protocol))

	return b.Bytes()
}

/*
IPv6パケットの受信処理
*/
func ipv6Input(inputdev *netDevice, packet []byte) {
	// IPアドレスのついていないインターフェースからの受信は無視
	if inputdev.ipdev.address == 0 {
		return
	}
	// IPv6の固定長より短かったらドロップ
	if len(packet) < 40 {
		fmt.Printf("Received IP packet too short from %s\n", inputdev.name)
		return
	}

	// 受信したIPv6パケットを構造体にセットする
	ipv6hader := ipv6Header{
		version:      packet[0] >> 4,
		trafficClass: packet[0] >> 7,
		flowLabel:    byteToUint32([]byte{packet[0] >> 7, packet[1], packet[2], packet[3]}),
		headerLen:    byteToUint16(packet[4:6]),
		nextHeader:   packet[6],
		hoplimit:     packet[7],
		srcAddr:      byteToUint64(packet[8:24]),
		destAddr:     byteToUint64(packet[24:40]),
	}

	fmt.Printf("ipv6 packet is %+v\n", ipv6hader)

	// 受信したMACアドレスがARPテーブルになければ追加しておく
	macaddr, _ := searchArpTableEntry(ipv6hader.srcAddr)
	if macaddr == [6]uint8{} {
		addArpTableEntry(inputdev, ipv6hader.srcAddr, inputdev.etheHeader.srcAddr)
	}
	// IPバージョンが6でなければドロップ
	if ipv6hader.version != 6 {
		fmt.Println("packet is not IPv6")
		return
	}
	// 宛先アドレスがブロードキャストアドレスか受信したNICインターフェイスのIPアドレスの場合
	if ipv6hader.destAddr == inputdev.ipdev.addressv6 {
		// 自分宛の通信として処理
		ipv6InputToOurs(inputdev, &ipv6hader, packet[40:])
	}
}

/*
自分宛てのIPv6パケットの処理
*/
func ipv6InputToOurs(inputdev *netDevice, ipheader *ipv6Header, packet []byte) {
	// 上位プロトコルの処理に移行
	switch ipheader.nextHeader {
	case IP_PROTOCOL_NUM_ICMPv6:
		fmt.Println("ICMPv6 received!")
		icmpv6Input(inputdev, ipheader.srcAddr, ipheader.destAddr, packet)
	case IP_PROTOCOL_NUM_UDP:
		fmt.Printf("udp received : %x\n", packet)
		//return
	case IP_PROTOCOL_NUM_TCP:
		return
	default:
		fmt.Printf("Unhandled ip protocol number : %d\n", ipheader.nextHeader)
		return
	}
}
