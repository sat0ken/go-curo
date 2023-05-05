package main

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

const IP_ADDRESS_LEN = 4
const IP_ADDRESS_LIMITED_BROADCAST uint32 = 0xffffffff
const IP_PROTOCOL_NUM_ICMP uint8 = 0x01
const IP_PROTOCOL_NUM_TCP uint8 = 0x06
const IP_PROTOCOL_NUM_UDP uint8 = 0x11

type ipDevice struct {
	address   uint32    // デバイスのIPアドレス
	netmask   uint32    // サブネットマスク
	broadcast uint32    // ブロードキャストアドレス
	natdev    natDevice // 5章で追加
}

type ipHeader struct {
	version        uint8  // バージョン
	headerLen      uint8  // ヘッダ長
	tos            uint8  // Type of Service
	totalLen       uint16 // Totalのパケット長
	identify       uint16 // 識別番号
	fragOffset     uint16 // フラグ
	ttl            uint8  // Time To Live
	protocol       uint8  // 上位のプロトコル番号
	headerChecksum uint16 // ヘッダのチェックサム
	srcAddr        uint32 // 送信元IPアドレス
	destAddr       uint32 // 送信先IPアドレス
}

type ipRouteType uint8

const (
	connected ipRouteType = iota
	network
)

type ipRouteEntry struct {
	iptype  ipRouteType
	netdev  *netDevice
	nexthop uint32
}

func (ipheader ipHeader) ToPacket(calc bool) (ipHeaderByte []byte) {
	var b bytes.Buffer

	b.Write([]byte{ipheader.version<<4 + ipheader.headerLen})
	b.Write([]byte{ipheader.tos})
	b.Write(uint16ToByte(ipheader.totalLen))
	b.Write(uint16ToByte(ipheader.identify))
	b.Write(uint16ToByte(ipheader.fragOffset))
	b.Write([]byte{ipheader.ttl})
	b.Write([]byte{ipheader.protocol})
	b.Write(uint16ToByte(ipheader.headerChecksum))
	b.Write(uint32ToByte(ipheader.srcAddr))
	b.Write(uint32ToByte(ipheader.destAddr))

	// checksumを計算する
	if calc {
		ipHeaderByte = b.Bytes()
		checksum := calcChecksum(ipHeaderByte)
		// checksumをセット
		ipHeaderByte[10] = checksum[0]
		ipHeaderByte[11] = checksum[1]
	} else {
		ipHeaderByte = b.Bytes()
	}

	return ipHeaderByte
}

func getIPdevice(addrs []net.Addr) (ipdev ipDevice) {
	for _, addr := range addrs {
		// ipv6ではなくipv4アドレスをリターン
		ipaddrstr := addr.String()
		if !strings.Contains(ipaddrstr, ":") && strings.Contains(ipaddrstr, ".") {
			ip, ipnet, _ := net.ParseCIDR(ipaddrstr)
			ipdev.address = byteToUint32(ip.To4())
			ipdev.netmask = byteToUint32(ipnet.Mask)
			// ブロードキャストアドレスの計算はIPアドレスとサブネットマスクのbit反転の2進数「OR（論理和）」演算
			ipdev.broadcast = ipdev.address | (^ipdev.netmask)
		}
	}
	return ipdev
}

func printIPAddr(ip uint32) string {
	ipbyte := uint32ToByte(ip)
	return fmt.Sprintf("%d.%d.%d.%d", ipbyte[0], ipbyte[1], ipbyte[2], ipbyte[3])
}

// サブネットマスクとプレフィックス長の変換
// 0xffffff00を24にする
func subnetToPrefixLen(netmask uint32) uint32 {
	var prefixlen uint32
	for prefixlen = 0; prefixlen < 32; prefixlen++ {
		if !(netmask>>(31-prefixlen)&0b01 == 1) {
			break
		}
	}
	return prefixlen
}

/*
IPパケットの受信処理
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/ip.cpp#L51
*/
func ipInput(inputdev *netDevice, packet []byte) {
	// IPアドレスのついていないインターフェースからの受信は無視
	if inputdev.ipdev.address == 0 {
		return
	}
	// IPヘッダ長より短かったらドロップ
	if len(packet) < 20 {
		fmt.Printf("Received IP packet too short from %s\n", inputdev.name)
		return
	}
	// 受信したIPパケットをipHeader構造体にセットする
	ipheader := ipHeader{
		version:        packet[0] >> 4,
		headerLen:      packet[0] << 5 >> 5,
		tos:            packet[1],
		totalLen:       byteToUint16(packet[2:4]),
		identify:       byteToUint16(packet[4:6]),
		fragOffset:     byteToUint16(packet[6:8]),
		ttl:            packet[8],
		protocol:       packet[9],
		headerChecksum: byteToUint16(packet[10:12]),
		srcAddr:        byteToUint32(packet[12:16]),
		destAddr:       byteToUint32(packet[16:20]),
	}

	fmt.Printf("ipInput Received IP in %s, packet type %d from %s to %s\n", inputdev.name, ipheader.protocol,
		printIPAddr(ipheader.srcAddr), printIPAddr(ipheader.destAddr))

	// 受信したMACアドレスがARPテーブルになければ追加しておく
	macaddr, _ := searchArpTableEntry(ipheader.srcAddr)
	if macaddr == [6]uint8{} {
		addArpTableEntry(inputdev, ipheader.srcAddr, inputdev.etheHeader.srcAddr)
	}

	// IPバージョンが4でなければドロップ
	// Todo: IPv6の実装
	if ipheader.version != 4 {
		if ipheader.version == 6 {
			fmt.Println("packet is IPv6")
		} else {
			fmt.Println("Incorrect IP version")
		}
		return
	}

	// IPヘッダオプションがついていたらドロップ = ヘッダ長が20byte以上だったら
	if 20 < (ipheader.headerLen * 4) {
		fmt.Println("IP header option is not supported")
		return
	}

	// 宛先アドレスがブロードキャストアドレスか受信したNICインターフェイスのIPアドレスの場合
	if ipheader.destAddr == IP_ADDRESS_LIMITED_BROADCAST || inputdev.ipdev.address == ipheader.destAddr {
		// 自分宛の通信として処理
		ipInputToOurs(inputdev, &ipheader, packet[20:])
		return
	}

	// 宛先IPアドレスをルータが持ってるか調べる
	// つまり宛先IPが他のNICインターフェイスについてるIPアドレスだったら自分宛てのものとして処理する
	for _, dev := range netDeviceList {
		// 宛先IPアドレスがルータの持っているIPアドレス or ディレクティッド・ブロードキャストアドレスの時の処理
		if dev.ipdev.address == ipheader.destAddr || dev.ipdev.broadcast == ipheader.destAddr {
			// 自分宛の通信として処理
			ipInputToOurs(inputdev, &ipheader, packet[20:])
			return
		}
	}

	// 5章で追加
	var natPacket []byte
	// NATの内側から外側への通信
	if inputdev.ipdev.natdev != (natDevice{}) {
		var err error
		switch ipheader.protocol {
		case IP_PROTOCOL_NUM_UDP:
			natPacket, err = natExec(&ipheader, natPacketHeader{packet: packet[20:]}, inputdev.ipdev.natdev, udp, outgoing)
			if err != nil {
				// NATできないパケットはドロップ
				fmt.Printf("nat udp packet err is %s\n", err)
				return
			}
		case IP_PROTOCOL_NUM_TCP:
			natPacket, err = natExec(&ipheader, natPacketHeader{packet: packet[20:]}, inputdev.ipdev.natdev, tcp, outgoing)
			if err != nil {
				// NATできないパケットはドロップ
				fmt.Printf("nat tcp packet err is %s\n", err)
				return
			}
		}
	}

	// 以下は4章で追加
	// 宛先IPアドレスがルータの持っているIPアドレスでない場合はフォワーディングを行う
	route := iproute.radixTreeSearch(ipheader.destAddr) // ルーティングテーブルをルックアップ
	if route == (ipRouteEntry{}) {
		// 宛先までの経路がなかったらパケットを破棄
		fmt.Printf("このIPへの経路がありません : %s\n", printIPAddr(ipheader.destAddr))
		return
	}

	// TTLが1以下ならドロップ
	if ipheader.ttl <= 1 {
		// Todo: send_icmp_time_exceeded関数を作成
		return
	}

	// TTLを1へらす
	ipheader.ttl -= 1

	// IPヘッダチェックサムの再計算
	ipheader.headerChecksum = 0
	ipheader.headerChecksum = byteToUint16(calcChecksum(ipheader.ToPacket(true)))

	// my_buf構造にコピー
	forwardPacket := ipheader.ToPacket(true)
	// NATの内側から外側への通信
	if inputdev.ipdev.natdev != (natDevice{}) {
		forwardPacket = append(forwardPacket, natPacket...)
	} else {
		forwardPacket = append(forwardPacket, packet[20:]...)
	}

	if route.iptype == connected { // 直接接続ネットワークの経路なら
		// hostに直接送信
		ipPacketOutputToHost(route.netdev, ipheader.destAddr, forwardPacket)
	} else { // 直接接続ネットワークの経路ではなかったら
		fmt.Printf("next hop is %s\n", printIPAddr(route.nexthop))
		fmt.Printf("forward packet is %x : %x\n", forwardPacket[0:20], natPacket)
		ipPacketOutputToNetxhop(route.nexthop, forwardPacket)
	}
}

/*
自分宛のIPパケットの処理
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/ip.cpp#L26
*/
func ipInputToOurs(inputdev *netDevice, ipheader *ipHeader, packet []byte) {
	// 5章で追加
	// NATの外側から内側への通信か判断
	for _, dev := range netDeviceList {
		if dev.ipdev != (ipDevice{}) && dev.ipdev.natdev != (natDevice{}) &&
			dev.ipdev.natdev.outsideIpAddr == ipheader.destAddr {
			// 送信先のIPがNATの外側のIPなら以下処理を実行
			// NATの戻りのパケットをDNATする
			natExecuted := false
			var destPacket []byte
			var err error
			switch ipheader.protocol {
			case IP_PROTOCOL_NUM_UDP:
				destPacket, err = natExec(ipheader, natPacketHeader{packet: packet}, dev.ipdev.natdev, udp, incoming)
				if err != nil {
					return
				}
				natExecuted = true
			case IP_PROTOCOL_NUM_TCP:
				destPacket, err = natExec(ipheader, natPacketHeader{packet: packet}, dev.ipdev.natdev, tcp, incoming)
				if err != nil {
					return
				}
				natExecuted = true
			}
			if natExecuted {
				ipPacket := ipheader.ToPacket(false)
				ipPacket = append(ipPacket, destPacket...)
				fmt.Printf("To dest is %s, checksum is %x, packet is %x\n", printIPAddr(ipheader.destAddr),
					ipheader.headerChecksum, ipPacket)
				ipPacketOutput(dev, iproute, ipheader.destAddr, ipPacket)
				return
			}
		}
	}
	// 上位プロトコルの処理に移行
	switch ipheader.protocol {
	case IP_PROTOCOL_NUM_ICMP:
		fmt.Println("ICMP received!")
		// 3章で追加
		icmpInput(inputdev, ipheader.srcAddr, ipheader.destAddr, packet)
	case IP_PROTOCOL_NUM_UDP:
		fmt.Printf("udp received : %x\n", packet)
		//return
	case IP_PROTOCOL_NUM_TCP:
		return
	default:
		fmt.Printf("Unhandled ip protocol number : %d\n", ipheader.protocol)
		return
	}
}

/*
IPパケットを直接イーサネットでホストに送信
*/
func ipPacketOutputToHost(dev *netDevice, destAddr uint32, packet []byte) {
	// ARPテーブルの検索
	destMacAddr, _ := searchArpTableEntry(destAddr)
	if destMacAddr == [6]uint8{0, 0, 0, 0, 0, 0} {
		// ARPエントリが無かったら
		fmt.Printf("Trying ip output to host, but no arp record to %s\n", printIPAddr(destAddr))
		// ARPリクエストを送信
		sendArpRequest(dev, destAddr)
	} else {
		// ARPエントリがあり、MACアドレスが得られたらイーサネットでカプセル化して送信
		ethernetOutput(dev, destMacAddr, packet, ETHER_TYPE_IP)
	}
}

/*
IPパケットをNextHopに送信
*/
func ipPacketOutputToNetxhop(nextHop uint32, packet []byte) {
	// ARPテーブルの検索
	destMacAddr, dev := searchArpTableEntry(nextHop)
	if destMacAddr == [6]uint8{0, 0, 0, 0, 0, 0} {
		fmt.Printf("Trying ip output to next hop, but no arp record to %s\n", printIPAddr(nextHop))
		// ルーティングテーブルのルックアップ
		routeToNexthop := iproute.radixTreeSearch(nextHop)
		//fmt.Printf("next hop route is from %s\n", routeToNexthop.netdev.name)
		if routeToNexthop == (ipRouteEntry{}) || routeToNexthop.iptype != connected {
			// next hopへの到達性が無かったら
			fmt.Printf("Next hop %s is not reachable\n", printIPAddr(nextHop))
		} else {
			// ARPリクエストを送信
			sendArpRequest(routeToNexthop.netdev, nextHop)
		}
	} else {
		// ARPエントリがあり、MACアドレスが得られたらイーサネットでカプセル化して送信
		ethernetOutput(dev, destMacAddr, packet, ETHER_TYPE_IP)
	}
}

/*
IPパケットを送信
*/
func ipPacketOutput(outputdev *netDevice, routeTree radixTreeNode, destAddr uint32, packet []byte) {
	// 宛先IPアドレスへの経路を検索
	route := routeTree.radixTreeSearch(destAddr)
	if route == (ipRouteEntry{}) {
		// 経路が見つからなかったら
		fmt.Printf("No route to %s\n", printIPAddr(destAddr))
	}
	if route.iptype == connected {
		// 直接接続されたネットワークなら
		ipPacketOutputToHost(outputdev, destAddr, packet)
	} else if route.iptype == network {
		// 直接つながっていないネットワークなら
		ipPacketOutputToNetxhop(destAddr, packet)
	}
}

/*
IPパケットにカプセル化して送信
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/ip.cpp#L102
*/
func ipPacketEncapsulateOutput(inputdev *netDevice, destAddr, srcAddr uint32, payload []byte, protocolType uint8) {
	var ipPacket []byte

	// IPヘッダで必要なIPパケットの全長を算出する
	// IPヘッダの20byte + パケットの長さ
	totalLength := 20 + len(payload)

	// IPヘッダの各項目を設定
	ipheader := ipHeader{
		version:        4,
		headerLen:      20 / 4,
		tos:            0,
		totalLen:       uint16(totalLength),
		identify:       0xf80c,
		fragOffset:     2 << 13,
		ttl:            0x40,
		protocol:       protocolType,
		headerChecksum: 0, // checksum計算する前は0をセット
		srcAddr:        srcAddr,
		destAddr:       destAddr,
	}
	// IPヘッダをByteにする
	ipPacket = append(ipPacket, ipheader.ToPacket(true)...)
	// payloadを追加
	ipPacket = append(ipPacket, payload...)

	// ルートテーブルを検索して送信先IPのMACアドレスがなければ、
	// ARPリクエストを生成して送信して結果を受信してから、ethernetからパケットを送る
	destMacAddr, _ := searchArpTableEntry(destAddr)
	if destMacAddr != [6]uint8{0, 0, 0, 0, 0, 0} {
		// ルートテーブルに送信するIPアドレスのMACアドレスがあれば送信
		ethernetOutput(inputdev, destMacAddr, ipPacket, ETHER_TYPE_IP)
	} else {
		// ARPリクエストを出す
		sendArpRequest(inputdev, destAddr)
	}
}
