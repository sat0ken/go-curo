package main

import (
	"bytes"
	"fmt"
)

const (
	// https://tex2e.github.io/rfc-translater/html/rfc4443.html
	ICMPv6_TYPE_ECHO_REQUEST           uint8 = 128
	ICMPv6_TYPE_ECHO_REPLY             uint8 = 129
	ICMPv6_TYPE_Router_Solicitation    uint8 = 133
	ICMPv6_TYPE_Router_Advertisement   uint8 = 134
	ICMPv6_TYPE_Neighbor_Solicitation  uint8 = 135
	ICMPv6_TYPE_Neighbor_Advertisement uint8 = 136

	// https://tex2e.github.io/rfc-translater/html/rfc4191.html
	// 2.1. Preference Values
	DefaultRouter_Preference_High   uint8 = 01
	DefaultRouter_Preference_Medium uint8 = 00
	DefaultRouter_Preference_Low    uint8 = 11
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

// https://tex2e.github.io/rfc-translater/html/rfc4861.html#4-1--Router-Solicitation-Message-Format
type routerSolicitation struct {
	reserved uint32
	optnd    optNeighborDiscovery
}

// https://tex2e.github.io/rfc-translater/html/rfc4861.html#4-2--Router-Advertisement-Message-Format
type routerAdvertisement struct {
	curhoplimit   uint8
	routerAdflags routerAdvertisementFlags
	lifetime      uint16
	reachabletime uint32
	retranstime   uint32
	options       []optNeighborDiscovery
}

type routerAdvertisementFlags struct {
	isManagedAddrConfig      bool
	isOtherConfig            bool
	isMobileHomeAgent        bool  // RFC3775
	isDefaultRouterPref      uint8 // RFC4191
	isNeighborDiscoveryProxy bool  // RFC4389
	reserved                 uint8
}

// https://tex2e.github.io/rfc-translater/html/rfc4861.html#4-3--Neighbor-Solicitation-Message-Format
type neighborSolicitation struct {
	reserved   uint32
	targetAddr [16]byte
	optnd      optNeighborDiscovery
}

// https://tex2e.github.io/rfc-translater/html/rfc4861.html#4-4--Neighbor-Advertisement-Message-Format
type neighborAdvertisement struct {
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
	options any
}

// 4.6.1. Source/Target Link-layer Address
type optLinkLayerAddr struct {
	macAddr [6]uint8
}

// 4.6.2. Prefix Information
type optPrefixInfomation struct {
	prefixLen          uint8
	flagOnLink         bool
	flagAutoAddrConfig bool
	validLifetime      uint32
	prefLifetime       uint32
	reserved           uint32
	prefix             [16]byte
}

func (optnd *optNeighborDiscovery) ToPacket() []byte {
	var b bytes.Buffer
	b.Write([]byte{optnd.opttype, optnd.length})
	switch optnd.opttype {
	case 1:
		b.Write(macToByte(optnd.options.(optLinkLayerAddr).macAddr))
	case 3:
		var flagbyte uint8
		prefixinfo := optnd.options.(optPrefixInfomation)
		if prefixinfo.flagOnLink {
			flagbyte += 128
		}
		if prefixinfo.flagAutoAddrConfig {
			flagbyte += 64
		}
		b.Write([]byte{prefixinfo.prefixLen, flagbyte})
		b.Write(uint32ToByte(prefixinfo.validLifetime))
		b.Write(uint32ToByte(prefixinfo.prefLifetime))
		b.Write(uint32ToByte(prefixinfo.reserved))
		b.Write(prefixinfo.prefix[:])
	}

	return b.Bytes()
}

func (ns *neighborSolicitation) ToPacket() []byte {
	var b bytes.Buffer
	b.Write(uint32ToByte(ns.reserved))
	b.Write(ns.targetAddr[:])
	b.Write(ns.optnd.ToPacket())

	return b.Bytes()
}

func (na *neighborAdvertisement) ToPacket() []byte {
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
	b.Write(macToByte(na.optnd.options.([6]uint8)))

	return b.Bytes()
}

func (ra *routerAdvertisement) ToPacket() []byte {
	var b bytes.Buffer
	b.Write([]byte{ra.curhoplimit})
	b.Write([]byte{ra.routerAdflags.ToPacket()})
	b.Write(uint16ToByte(ra.lifetime))
	b.Write(uint32ToByte(ra.reachabletime))
	b.Write(uint32ToByte(ra.retranstime))

	for _, opt := range ra.options {
		packet := opt.ToPacket()
		b.Write(packet)
	}

	return b.Bytes()
}

func (rdaFlags *routerAdvertisementFlags) ToPacket() uint8 {
	if rdaFlags.isManagedAddrConfig {
		rdaFlags.reserved += 128
	}
	if rdaFlags.isOtherConfig {
		rdaFlags.reserved += 64
	}
	if rdaFlags.isMobileHomeAgent {
		rdaFlags.reserved += 32
	}
	// Highだったら01なので8を足す。Mediumは00なので足す必要はない
	if rdaFlags.isDefaultRouterPref == DefaultRouter_Preference_High {
		rdaFlags.reserved += 8
	}
	if rdaFlags.isNeighborDiscoveryProxy {
		rdaFlags.reserved += 4
	}
	return rdaFlags.reserved
}

func (icmpmsg *icmpv6Message) ReplyEchoPacket(sourceAddr, destAddr [16]byte) (icmpv6Packet []byte) {
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
	na := neighborAdvertisement{
		flagRouter:    false,
		flagSolicited: true,
		flagOverRide:  true,
		reserved:      0,
		targetAddr:    icmpmsg.message.(neighborSolicitation).targetAddr,
		optnd: optNeighborDiscovery{
			opttype: 2,
			length:  1,
			options: sourceMacAddr,
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

func (icmpmsg *icmpv6Message) ReplyRouterAdvertisement(sourceAddr, destAddr, prefixAddr [16]byte, sourceMacAddr [6]uint8) (icmpv6Packet []byte) {
	var b bytes.Buffer
	// ICMPv6ヘッダ
	b.Write([]byte{ICMPv6_TYPE_Router_Advertisement})
	b.Write([]byte{0x00})       // icmp code
	b.Write([]byte{0x00, 0x00}) // checksum
	// Router Advertisementメッセージ
	ra := routerAdvertisement{
		curhoplimit: 64,
		routerAdflags: routerAdvertisementFlags{
			isManagedAddrConfig:      true,
			isOtherConfig:            true,
			isMobileHomeAgent:        false,
			isDefaultRouterPref:      DefaultRouter_Preference_Medium,
			isNeighborDiscoveryProxy: false,
			reserved:                 0,
		},
		lifetime:      1800,
		reachabletime: 0,
		retranstime:   0,
		options: []optNeighborDiscovery{
			// Prefix Information
			{
				opttype: 3,
				length:  4,
				options: optPrefixInfomation{
					prefixLen:          64,
					flagOnLink:         true,
					flagAutoAddrConfig: true,
					validLifetime:      86400,
					prefLifetime:       14400,
					reserved:           0,
					prefix:             prefixAddr,
				},
			},
			// Source Link Layer Address
			{
				opttype: 1,
				length:  1,
				options: optLinkLayerAddr{macAddr: sourceMacAddr},
			},
		},
	}
	b.Write(ra.ToPacket())
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

func (icmpmsg *icmpv6Message) ReplyNeighborSolicitation(sourceAddr, destAddr [16]byte, sourceMacAddr [6]uint8) (icmpv6Packet []byte) {
	var b bytes.Buffer
	// ICMPv6ヘッダ
	b.Write([]byte{ICMPv6_TYPE_Neighbor_Solicitation})
	b.Write([]byte{0x00})       // icmp code
	b.Write([]byte{0x00, 0x00}) // checksum
	// Neighbor Solicitationメッセージ
	ns := neighborSolicitation{
		reserved:   0,
		targetAddr: destAddr,
		optnd: optNeighborDiscovery{
			opttype: 1,
			length:  1,
			options: setMacAddr(sourceMacAddr[:]),
		},
	}
	b.Write(ns.ToPacket())
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
		payload := icmpmsg.ReplyEchoPacket(sourceAddr, destAddr)
		ipv6PacketEncapsulateOutput(inputdev, sourceAddr, destAddr, payload, IP_PROTOCOL_NUM_ICMPv6)
	case ICMPv6_TYPE_Router_Solicitation:
		fmt.Println("ICMPv6 Router_Solicitation is received")
		var prefixAddr [16]byte
		icmpmsg.message = routerSolicitation{
			reserved: byteToUint32(icmpPacket[4:8]),
		}
		for _, addr := range *inputdev.ipdev.ipv6AddrList {
			if !bytes.HasPrefix(addr.v6address[:], []byte{0xfe, 0x80}) {
				prefixAddr = getPrefixIpv6(addr.v6address, addr.prefix)
			} else {
				destAddr = addr.v6address
			}
		}
		payload := icmpmsg.ReplyRouterAdvertisement(sourceAddr, destAddr, prefixAddr, inputdev.macaddr)
		fmt.Printf("payload is %x\n", payload)
		ipv6PacketEncapsulateOutput(inputdev, sourceAddr, destAddr, payload, IP_PROTOCOL_NUM_ICMPv6)
	case ICMPv6_TYPE_Neighbor_Solicitation:
		fmt.Println("ICMPv6 Neighbor_Solicitation is received")
		icmpmsg.message = neighborSolicitation{
			reserved:   byteToUint32(icmpPacket[4:8]),
			targetAddr: setipv6addr(icmpPacket[8:24]),
			optnd: optNeighborDiscovery{
				opttype: icmpPacket[25],
				length:  icmpPacket[26],
				options: setMacAddr(icmpPacket[26:32]),
			},
		}
		payload := icmpmsg.ReplyNeighborAdvertisement(sourceAddr, destAddr, inputdev.macaddr)
		ipv6PacketEncapsulateOutput(inputdev, sourceAddr, destAddr, payload, IP_PROTOCOL_NUM_ICMPv6)
	}
}

func sendNeighborSolicitation(netdev *netDevice, ipv6 ipv6Header) {
	var ipv6Packet []byte

	fmt.Printf("Sending arp request via %s for %x\n", netdev.name, ipv6.destAddr)
	icmpmsg := icmpv6Message{
		icmpType: ICMPv6_TYPE_Neighbor_Solicitation,
		icmpCode: 0,
		message: neighborSolicitation{
			reserved:   0,
			targetAddr: ipv6.destAddr,
			optnd: optNeighborDiscovery{
				opttype: 1,
				length:  1,
				options: setMacAddr(netdev.macaddr[:]),
			},
		},
	}
	// IPv6ヘッダをパケットにする
	ipv6Packet = append(ipv6Packet, ipv6.ToPacket()...)
	// payloadを追加
	payload := icmpmsg.ReplyNeighborSolicitation(ipv6.srcAddr, ipv6.destAddr, netdev.macaddr)
	ipv6Packet = append(ipv6Packet, payload...)
	// ethernetでカプセル化して送信
	destMacAddr := [6]uint8{0x33, 0x33, 0xff, ipv6.destAddr[13], ipv6.destAddr[14], ipv6.destAddr[15]}
	ethernetOutput(netdev, destMacAddr, payload, ETHER_TYPE_IPV6)
}
