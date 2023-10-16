package main

import (
	"fmt"
	"math/rand"
)

var nat64PrefixAddr = []byte{0x00, 0x64, 0xff, 0x9b}

type nat64Entry struct {
	destipv4Addr uint32
	srcipv6Addr  [16]byte
	destipv6Addr [16]byte
	protcolType  int
	icmpIdentify uint16
}

type nat64EntryList struct {
	icmp []*nat64Entry
}

// NATの内側のip_deviceが持つNATデバイス
type nat64Device struct {
	outsideIpAddr uint32
	nat64Entry    *nat64EntryList
}

func configureIPNat64(inside string, outside uint32) {

	for _, dev := range netDeviceList {
		if inside == dev.name {
			dev.ipdev.natdev = natDevice{
				outsideIpAddr: outside,
				nat64Entry: &nat64EntryList{
					icmp: make([]*nat64Entry, NAT_ICMP_ID_SIZE, NAT_ICMP_ID_SIZE),
				},
			}
			fmt.Printf("Set nat to %s, outside ip addr is %s\n", inside, printIPAddr(outside))
		}
	}
}

func ipv6HeaderToipv4Header(ipv6header *ipv6Header, srcAddr, destAddr uint32) ipHeader {
	ipv4Header := ipHeader{
		version:        4,
		headerLen:      20 / 4,
		tos:            0,
		totalLen:       ipv6header.headerLen + 20,
		identify:       uint16(rand.Intn(65535)),
		fragOffset:     0,
		ttl:            ipv6header.hoplimit - 1,
		protocol:       0,
		headerChecksum: 0,
		srcAddr:        srcAddr,
		destAddr:       destAddr,
	}
	if ipv6header.nextHeader == IP_PROTOCOL_NUM_ICMPv6 {
		ipv4Header.protocol = IP_PROTOCOL_NUM_ICMP
	}
	return ipv4Header
}

func nat6to4Exec(inputdev *netDevice, ipv6header *ipv6Header, packet []byte) {
	var ipPacket []byte

	// ルーティングテーブルをルックアップ
	destIpv4Addr := byteToUint32(ipv6header.destAddr[12:16])
	route := iproute.radixTreeSearch(destIpv4Addr)
	if route == (ipRouteEntry{}) {
		// 宛先までの経路がなかったらパケットを破棄
		fmt.Printf("このIPへの経路がありません : %s\n", printIPAddr(destIpv4Addr))
		return
	}
	// IPv6ヘッダをIPv4ヘッダに変換
	ipv4Header := ipv6HeaderToipv4Header(ipv6header, route.netdev.ipdev.address, destIpv4Addr)

	var icmpmsg icmpv6Message
	if ipv6header.nextHeader == IP_PROTOCOL_NUM_ICMPv6 {
		icmpmsg = icmpv6Message{
			icmpType: packet[0],
			icmpCode: packet[1],
			checksum: byteToUint16(packet[2:4]),
		}
		icmpmsg.message = icmpEcho{
			identify: byteToUint16(packet[4:6]),
			sequence: byteToUint16(packet[6:8]),
			data:     packet[8:],
		}
	}

	// IPヘッダをByteにする
	ipPacket = append(ipPacket, ipv4Header.ToPacket(true)...)
	// payloadを追加
	ipPacket = append(ipPacket, icmpmsg.ToICMPv4Packet()...)

	fmt.Printf("NAT64 Packet is %x\n", ipPacket)

	destMacAddr, _ := searchArpTableEntry(destIpv4Addr)
	fmt.Printf("NAT64 destMacAddr is %s\n", printMacAddr(destMacAddr))
	if destMacAddr != [6]uint8{0, 0, 0, 0, 0, 0} {
		// Todo: 送信前にNATエントリに追加

		// ARPテーブルに送信するIPアドレスのMACアドレスがあれば送信
		fmt.Printf("NAT64 Ethernet output is %x\n", ipPacket)
		ethernetOutput(route.netdev, destMacAddr, ipPacket, ETHER_TYPE_IP)
	} else {
		// ARPリクエストを出す
		fmt.Printf("NAT64 ARP Request to %s\n", printIPAddr(destIpv4Addr))
		sendArpRequest(route.netdev, destIpv4Addr)
	}
}
