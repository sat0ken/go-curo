package main

import (
	"bytes"
	"fmt"
)

const FLOW_LABEL = 0x123456

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

func (ipv6Header *ipv6Header) ToPacket() []byte {
	var b bytes.Buffer

	b.Write([]byte{ipv6Header.version << 4})
	b.Write([]byte{ipv6Header.trafficClass})
	b.Write(uint32ToByte(ipv6Header.flowLabel))
	b.Write(uint16ToByte(ipv6Header.headerLen))
	b.Write([]byte{ipv6Header.nextHeader})
	b.Write([]byte{ipv6Header.hoplimit})
	b.Write(uint64ToByte(ipv6Header.srcAddr))
	b.Write(uint64ToByte(ipv6Header.destAddr))

	return b.Bytes()
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
	ipv6header := ipv6Header{
		version:      packet[0] >> 4,
		trafficClass: packet[0] >> 7,
		flowLabel:    byteToUint32([]byte{packet[0] >> 7, packet[1], packet[2], packet[3]}),
		headerLen:    byteToUint16(packet[4:6]),
		nextHeader:   packet[6],
		hoplimit:     packet[7],
		srcAddr:      byteToUint64(packet[8:24]),
		destAddr:     byteToUint64(packet[24:40]),
	}

	fmt.Printf("ipv6 packet is %+v\n", ipv6header)

	// 受信したMACアドレスがARPテーブルになければ追加しておく
	macaddr, _ := searchArpTableEntry(ipv6header.srcAddr)
	if macaddr == [6]uint8{} {
		addArpTableEntry(inputdev, ipv6header.srcAddr, inputdev.etheHeader.srcAddr)
	}
	// IPバージョンが6でなければドロップ
	if ipv6header.version != 6 {
		fmt.Println("packet is not IPv6")
		return
	}
	// 宛先アドレスがブロードキャストアドレスか受信したNICインターフェイスのIPアドレスの場合
	if ipv6header.destAddr == inputdev.ipdev.addressv6 {
		// 自分宛の通信として処理
		ipv6InputToOurs(inputdev, &ipv6header, packet[40:])
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

/*
IPv6パケットにカプセル化して送信
*/
func ipv6PacketEncapsulateOutput(inputdev *netDevice, destAddr, srcAddr uint64, payload []byte, protocolType uint8) {
	var ipv6Packet []byte

	// IPv6ヘッダで必要なIPパケットの全長を算出する
	// IPv6ヘッダの40byte + パケットの長さ
	totalLength := 40 + len(payload)

	// IPv6ヘッダの各項目を設定
	ipv6Header := ipv6Header{
		version:      6,
		trafficClass: 0,
		flowLabel:    FLOW_LABEL,
		headerLen:    uint16(totalLength),
		nextHeader:   IP_PROTOCOL_NUM_ICMPv6,
		hoplimit:     64,
		srcAddr:      srcAddr,
		destAddr:     destAddr,
	}
	// IPv6ヘッダをパケットにする
	ipv6Packet = append(ipv6Packet, ipv6Header.ToPacket()...)
	// payloadを追加
	ipv6Packet = append(ipv6Packet, payload...)
	// ルートテーブルを検索して送信先IPのMACアドレスがなければ、
	// ARPリクエストを生成して送信して結果を受信してから、ethernetからパケットを送る
	destMacAddr, _ := searchArpTableEntry(destAddr)
	if destMacAddr != [6]uint8{0, 0, 0, 0, 0, 0} {
		// ARPテーブルに送信するIPアドレスのMACアドレスがあれば送信
		ethernetOutput(inputdev, destMacAddr, ipv6Packet, ETHER_TYPE_IP)
	} else {
		// Todo: 近隣探索のパケットを出す
	}

}
