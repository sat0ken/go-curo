package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

const FLOW_LABEL = 0x137ca

// プロフェッショナルIPv6 12.3 Solicited-Nodeマルチキャストアドレス
var solicitedNoneMultiCastAddr = []byte{0xff, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0xff}

type ipv6Header struct {
	version      uint8
	trafficClass uint8
	flowLabel    uint32
	headerLen    uint16
	nextHeader   uint8
	hoplimit     uint8
	srcAddr      [16]byte
	destAddr     [16]byte
}

func (ipv6Header *ipv6Header) ToPacket() []byte {
	var b bytes.Buffer

	b.Write([]byte{ipv6Header.version << 4})
	//b.Write([]byte{ipv6Header.trafficClass})
	b.Write(uint32ToByte(ipv6Header.flowLabel)[1:])
	b.Write(uint16ToByte(ipv6Header.headerLen))
	b.Write([]byte{ipv6Header.nextHeader})
	b.Write([]byte{ipv6Header.hoplimit})
	b.Write(ipv6Header.srcAddr[:])
	b.Write(ipv6Header.destAddr[:])

	return b.Bytes()
}

// チェックサム計算用の疑似ヘッダ
type ipv6DummyHeader struct {
	srcAddr  [16]byte
	destAddr [16]byte
	length   uint32
	protocol uint32
}

func (ipv6Dummy *ipv6DummyHeader) ToPacket() []byte {
	var b bytes.Buffer

	b.Write(ipv6Dummy.srcAddr[:])
	b.Write(ipv6Dummy.destAddr[:])
	b.Write(uint32ToByte(ipv6Dummy.length))
	b.Write(uint32ToByte(ipv6Dummy.protocol))

	return b.Bytes()
}

func ipv6ToByte(addr string) (ipv6AddrByte [16]byte) {
	var b bytes.Buffer
	splitAddr := strings.Split(addr, ":")
	for _, v := range splitAddr {
		switch len(v) {
		case 3:
			v = fmt.Sprintf("0%s", v)
		case 2:
			v = fmt.Sprintf("00%s", v)
		case 1:
			v = fmt.Sprintf("000%s", v)
		case 0:
			for i := len(splitAddr); i <= 8; i++ {
				v += "0000"
			}
		}
		h, _ := hex.DecodeString(v)
		b.Write(h)
	}
	copy(ipv6AddrByte[:], b.Bytes())
	return ipv6AddrByte
}

func setipv6addr(ipv6 []byte) [16]byte {
	var b16 [16]byte
	for i, v := range ipv6 {
		b16[i] = v
	}
	return b16
}

func getPrefixIpv6(ipv6, prefix [16]byte) [16]byte {
	for i, _ := range ipv6 {
		if prefix[i] == 00 {
			ipv6[i] = 0x00
		}
	}
	return ipv6
}

/*
IPv6パケットの受信処理
*/
func ipv6Input(inputdev *netDevice, packet []byte) {

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
		srcAddr:      setipv6addr(packet[8:24]),
		destAddr:     setipv6addr(packet[24:40]),
	}

	fmt.Printf("ipv6 header is %+v\n", ipv6header)

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
	// 宛先アドレスがマルチキャストアドレスか受信したNICインターフェイスのIPv6アドレスの場合
	for _, ipv6addr := range *inputdev.ipdev.ipv6AddrList {
		if bytes.Equal(ipv6header.destAddr[:], ipv6addr.v6address[:]) {
			// 自分宛の通信として処理
			ipv6InputToOurs(inputdev, &ipv6header, packet[40:])
		} else if bytes.HasPrefix(ipv6header.destAddr[:], []byte{0xff, 0x02}) &&
			bytes.HasPrefix(ipv6addr.v6address[:], []byte{0xfe, 0x80}) {
			// 自分宛の通信として処理
			ipv6InputToOurs(inputdev, &ipv6header, packet[40:])
		}
	}

	// ipv6本 5章で追加
	// 宛先IPアドレスがルータの持っているIPアドレスでない場合はフォワーディングを行う
	route := iproute.radixTreeSearchv6(byteToUint64(ipv6header.destAddr[0:8])) // ルーティングテーブルをルックアップ
	if route == (ipRouteEntry{}) {
		// 宛先までの経路がなかったらパケットを破棄
		fmt.Printf("このIPへの経路がありません : %x\n", ipv6header.destAddr)
		return
	}
	// hop limitが0ならパケットを廃棄
	if ipv6header.hoplimit == 0 {
		return
	}
	// hop limitを1減らす
	ipv6header.hoplimit -= 1
	// パケットを再生成
	forwardPacket := ipv6header.ToPacket()
	forwardPacket = append(forwardPacket, packet[40:]...)
	// パケットを転送
	if route.iptype == connected { // 直接接続ネットワークの経路なら
		ipv6PacketOutputToHost(route.netdev, ipv6header, forwardPacket)
	} else { // 直接接続ネットワークの経路ではなかったら
		ipv6PacketOutputToNetxhop(route.nexthopv6, ipv6header, forwardPacket)
	}
}

/*
自分宛てのIPv6パケットの処理
*/
func ipv6InputToOurs(inputdev *netDevice, ipheader *ipv6Header, packet []byte) {
	// 上位プロトコルの処理に移行
	switch ipheader.nextHeader {
	case IP_PROTOCOL_NUM_ICMPv6:
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
func ipv6PacketEncapsulateOutput(inputdev *netDevice, destAddr, srcAddr [16]byte, payload []byte, protocolType uint8) {
	var ipv6Packet []byte

	// IPv6ヘッダの各項目を設定
	ipv6 := ipv6Header{
		version:      6,
		trafficClass: 0,
		flowLabel:    FLOW_LABEL,
		headerLen:    uint16(len(payload)),
		nextHeader:   protocolType,
		hoplimit:     255,
		srcAddr:      srcAddr,
		destAddr:     destAddr,
	}
	// IPv6ヘッダをパケットにする
	ipv6Packet = append(ipv6Packet, ipv6.ToPacket()...)
	// payloadを追加
	ipv6Packet = append(ipv6Packet, payload...)
	// ルートテーブルを検索して送信先IPのMACアドレスがなければ、
	// ARPリクエストを生成して送信して結果を受信してから、ethernetからパケットを送る
	destMacAddr, _ := searchArpTableEntry(destAddr)
	if destMacAddr != [6]uint8{0, 0, 0, 0, 0, 0} {
		// ARPテーブルに送信するIPアドレスのMACアドレスがあれば送信
		ethernetOutput(inputdev, destMacAddr, ipv6Packet, ETHER_TYPE_IPV6)
	} else {
		// 近隣探索のリクエストを出す
		sendNeighborSolicitation(inputdev, ipv6)
	}
}

/*
IPパケットを直接イーサネットでホストに送信
*/
func ipv6PacketOutputToHost(dev *netDevice, ipv6 ipv6Header, packet []byte) {
	// ARPテーブルの検索
	destMacAddr, _ := searchArpTableEntry(ipv6.destAddr)
	if destMacAddr == [6]uint8{0, 0, 0, 0, 0, 0} {
		// ARPリクエストを送信
		sendNeighborSolicitation(dev, ipv6)
	} else {
		// ARPエントリがあり、MACアドレスが得られたらイーサネットでカプセル化して送信
		ethernetOutput(dev, destMacAddr, packet, ETHER_TYPE_IPV6)
	}
}

/*
IPv6パケットをNextHopに送信
*/
func ipv6PacketOutputToNetxhop(nexthopv6 [16]byte, ipv6 ipv6Header, packet []byte) {
	// ARPテーブルの検索
	destMacAddr, dev := searchArpTableEntry(nexthopv6)
	fmt.Printf("destMacAddr is %x\n", destMacAddr)
	if destMacAddr == [6]uint8{0, 0, 0, 0, 0, 0} {
		// ルーティングテーブルのルックアップ
		routeToNexthopIPv6 := iproute.radixTreeSearchv6(byteToUint64(nexthopv6[0:8]))
		if routeToNexthopIPv6 == (ipRouteEntry{}) || routeToNexthopIPv6.iptype != connected {
			// next hopへの到達性が無かったら
			fmt.Printf("Next hop %x is not reachable\n", nexthopv6)
		} else {
			// 近隣探索のリクエストを出す
			ipv6.destAddr = nexthopv6
			sendNeighborSolicitation(routeToNexthopIPv6.netdev, ipv6)
		}
	} else {
		// ARPエントリがあり、MACアドレスが得られたらイーサネットでカプセル化して送信
		ethernetOutput(dev, destMacAddr, packet, ETHER_TYPE_IPV6)
	}
}
