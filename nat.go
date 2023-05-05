package main

import (
	"fmt"
)

type natDirectionType uint8
type natProtocolType uint8

const (
	incoming natDirectionType = iota
	outgoing
)

const (
	tcp natProtocolType = iota
	udp
	icmp
)

const (
	NAT_GLOBAL_PORT_MIN  = 20000
	NAT_GLOBAL_PORT_MAX  = 59999
	NAT_GLOBAL_PORT_SIZE = (NAT_GLOBAL_PORT_MAX - NAT_GLOBAL_PORT_MIN + 1)
	NAT_ICMP_ID_SIZE     = 0xffff
)

type natPacketHeader struct {
	// TCPヘッダかUDPヘッダかICMP
	packet []byte
}
type natEntry struct {
	globalIpAddr uint32
	localIpAddr  uint32
	globalPort   uint16
	localPort    uint16
}

// UDP, TCPのNATテーブルのセット
type natEntryList struct {
	tcp []*natEntry
	udp []*natEntry
}

// NATの内側のip_deviceが持つNATデバイス
type natDevice struct {
	outsideIpAddr uint32
	natEntry      *natEntryList
}

func configureIPNat(inside string, outside uint32) {

	for _, dev := range netDeviceList {
		if inside == dev.name {
			dev.ipdev.natdev = natDevice{
				outsideIpAddr: outside,
				natEntry: &natEntryList{
					tcp: make([]*natEntry, NAT_GLOBAL_PORT_SIZE, NAT_GLOBAL_PORT_SIZE),
					udp: make([]*natEntry, NAT_GLOBAL_PORT_SIZE, NAT_GLOBAL_PORT_SIZE),
				},
			}
			fmt.Printf("Set nat to %s, outside ip addr is %s\n", inside, printIPAddr(outside))
		}
	}
}

func dumpNatTables() {
	fmt.Println("|-PROTO-|---------LOCAL---------|--------GLOBAL---------|")
	for _, netdev := range netDeviceList {
		if netdev.ipdev != (ipDevice{}) && netdev.ipdev.natdev != (natDevice{}) {
			for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
				if netdev.ipdev.natdev.natEntry.tcp[i].globalPort != 0 {
					fmt.Printf("|  TCP  | %15d:%05d | %15d:%05d |\n",
						netdev.ipdev.natdev.natEntry.tcp[i].localIpAddr,
						netdev.ipdev.natdev.natEntry.tcp[i].localPort,
						netdev.ipdev.natdev.natEntry.tcp[i].globalIpAddr,
						netdev.ipdev.natdev.natEntry.tcp[i].globalPort)
				}
				if netdev.ipdev.natdev.natEntry.udp[i].globalPort != 0 {
					fmt.Printf("|  UDP  | %15s:%05d | %15s:%05d |\n",
						netdev.ipdev.natdev.natEntry.udp[i].localIpAddr,
						netdev.ipdev.natdev.natEntry.udp[i].localPort,
						netdev.ipdev.natdev.natEntry.udp[i].globalIpAddr,
						netdev.ipdev.natdev.natEntry.udp[i].globalPort)
				}
			}
		}
	}
	fmt.Println("|-------|-----------------------|-----------------------|")
}

/*
NATのアドレス変換を実行する
*/
func natExec(ipheader *ipHeader, natPacket natPacketHeader, natdevice natDevice, proto natProtocolType, direction natDirectionType) ([]byte, error) {

	var udpheader udpHeader
	var tcpheader tcpHeader
	var srcPort, destPort uint16
	var packet []byte
	var checksum uint32
	var ipchecksum uint32

	// プロトコルごとに型を変換
	switch proto {
	case udp:
		udpheader = udpheader.ParsePacket(natPacket.packet)
		srcPort = udpheader.srcPort
		destPort = udpheader.destPort
	case tcp:
		tcpheader = tcpheader.ParsePacket(natPacket.packet)
		srcPort = tcpheader.srcPort
		destPort = tcpheader.destPort
	}

	// ICMPのNATは未対応
	if proto == icmp {
		return nil, fmt.Errorf("ICMPのNATは未対応です...")
	}

	var entry *natEntry
	if direction == incoming { // NATの外から内への通信時
		// UDPとTCPの時はポート番号
		entry = natdevice.natEntry.getNatEntryByGlobal(proto, ipheader.destAddr, destPort)
		// NATエントリが登録されていない場合、エラーを返す
		if entry == (&natEntry{}) {
			return nil, fmt.Errorf("No nat entry")
		}
		fmt.Printf("incoming nat from %s:%d to %s:%d\n",
			printIPAddr(entry.globalIpAddr), entry.globalPort, printIPAddr(entry.localIpAddr), entry.localPort)

		fmt.Printf("incoming ip header src is %s, dest is %s\n",
			printIPAddr(ipheader.srcAddr), printIPAddr(ipheader.destAddr))
		// IPヘッダの送信先アドレスをentryのアドレスにする
		ipheader.destAddr = entry.localIpAddr
		tcpheader.destPort = entry.localPort

	} else { // NATの内から外への通信時

		entry = natdevice.natEntry.getNatEntryByLocal(proto, ipheader.srcAddr, srcPort)

		if entry.globalPort == 0 {
			// NATエントリがなかったらエントリ作成
			entry = natdevice.natEntry.createNatEntry(proto)
			if entry == (&natEntry{}) {
				return nil, fmt.Errorf("NAT table is full")
			}
			entry.globalIpAddr = natdevice.outsideIpAddr
			entry.localIpAddr = ipheader.srcAddr

			if proto == udp {
				entry.localPort = udpheader.srcPort
			} else {
				entry.localPort = tcpheader.srcPort
			}

			fmt.Printf("Now, nat entry local %s:%d to global %s:%d\n",
				printIPAddr(entry.localIpAddr), entry.localPort, printIPAddr(entry.globalIpAddr), entry.globalPort)
		}

		// IPヘッダの送信元アドレスを外側のアドレスにする
		ipheader.srcAddr = entry.globalIpAddr
		tcpheader.srcPort = entry.globalPort
	}

	if proto == udp {
		checksum = uint32(udpheader.checksum)
	} else {
		checksum = uint32(tcpheader.checksum)
	}
	// 反転前の1の補数和に戻す
	checksum = checksum ^ 0xffff
	ipchecksum = uint32(ipheader.headerChecksum ^ 0xffff)
	if direction == incoming {
		// destinationのIPアドレスを引く
		checksum += (entry.localIpAddr - entry.globalIpAddr)
		checksum += uint32(entry.localPort - entry.globalPort)
		// 桁あふれた1の補数を足し込む
		checksum = (checksum & 0xffff) + checksum>>16
		ipchecksum += (entry.localIpAddr - entry.globalIpAddr)
		ipheader.headerChecksum = uint16(ipchecksum ^ 0xffff)
	} else {
		// sourceのIPアドレスを引く
		checksum -= (entry.localIpAddr - entry.globalIpAddr)
		checksum -= uint32(entry.localPort - entry.globalPort)
		// 桁あふれた1の補数を足し込む
		checksum = (checksum & 0xffff) + checksum>>16
		ipchecksum -= (entry.localIpAddr - entry.globalIpAddr)
	}

	// 計算し直したchecksumをパケットにつけ直す
	if proto == udp {
		udpheader.checksum = uint16(checksum ^ 0xffff)
		packet = udpheader.ToPacket()
	} else {
		tcpheader.checksum = uint16(checksum ^ 0xffff)
		packet = tcpheader.ToPacket()
	}

	return packet, nil
}

/*
グローバルアドレスとグローバルポートからNATエントリを取得
*/
func (entry *natEntryList) getNatEntryByGlobal(protoType natProtocolType, ipaddr uint32, port uint16) *natEntry {
	fmt.Printf("getNatEntryByGlobal ipaddr is %s, port is %d\n", printIPAddr(ipaddr), port)
	switch protoType {
	case udp: // udpの場合
		for _, v := range entry.udp {
			if v != nil && v.globalIpAddr == ipaddr && v.globalPort == port {
				return v
			}
		}
	case tcp: // tcpの場合
		// TCPのNATテーブルをグローバルIPアドレス, ローカルポートで検索する
		for _, v := range entry.tcp {
			if v != nil && v.globalIpAddr == ipaddr && v.globalPort == port {
				return v
			}
		}
	}
	return &natEntry{}
}

/*
ローカルアドレスとローカルポートからNATエントリを取得
*/
func (entry *natEntryList) getNatEntryByLocal(protoType natProtocolType, ipaddr uint32, port uint16) *natEntry {

	switch protoType {
	case udp: // udpの場合
		// UDPのNATテーブルをローカルIPアドレス, ローカルポートで検索する
		for _, v := range entry.udp {
			if v != nil && v.localIpAddr == ipaddr && v.localPort == port {
				return v
			}
		}
	case tcp: // tcpの場合
		// TCPのNATテーブルをローカルIPアドレス, ローカルポートで検索する
		for _, v := range entry.tcp {
			if v != nil && v.localIpAddr == ipaddr && v.localPort == port {
				return v
			}
		}
	}
	// テーブルに一致するエントリがなかったらnullを返す
	return &natEntry{}
}

/*
空いてるポートを探し、NATエントリを作成する
*/
func (entry *natEntryList) createNatEntry(protoType natProtocolType) *natEntry {
	switch protoType {
	case udp:
		// udpの場合
		for i, v := range entry.udp {
			// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
			if v == nil {
				entry.udp[i] = &natEntry{
					globalPort: uint16(NAT_GLOBAL_PORT_MIN + i),
				}
				return entry.udp[i]
			}
		}
	case tcp:
		// tcpの場合
		for i, v := range entry.tcp {
			// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
			if v == nil {
				entry.tcp[i] = &natEntry{
					globalPort: uint16(NAT_GLOBAL_PORT_MIN + i),
				}
				return entry.tcp[i]
			}
		}
	}
	// 空いているエントリがなかったらnullを返す
	return &natEntry{}
}
