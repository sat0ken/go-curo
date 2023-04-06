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

// ICMP, UDP, TCPのNATテーブルのセット
type natEntryList struct {
	icmp []*natEntry
	tcp  []*natEntry
	udp  []*natEntry
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
					icmp: make([]*natEntry, NAT_ICMP_ID_SIZE, NAT_ICMP_ID_SIZE),
					tcp:  make([]*natEntry, NAT_GLOBAL_PORT_SIZE, NAT_GLOBAL_PORT_SIZE),
					udp:  make([]*natEntry, NAT_GLOBAL_PORT_SIZE, NAT_GLOBAL_PORT_SIZE),
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
			for i := 0; i < NAT_ICMP_ID_SIZE; i++ {
				if netdev.ipdev.natdev.natEntry.icmp[i].localIpAddr != 0 {
					fmt.Printf("|  UDP  | %15s:%05d | %15s:%05d |\n",
						netdev.ipdev.natdev.natEntry.icmp[i].localIpAddr,
						netdev.ipdev.natdev.natEntry.icmp[i].localPort,
						netdev.ipdev.natdev.natEntry.icmp[i].globalIpAddr,
						netdev.ipdev.natdev.natEntry.icmp[i].globalPort)
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
	var icmpmessage icmpMessage
	var udpheader udpHeader
	var tcpheader tcpHeader
	var srcPort, destPort uint16
	var packet []byte
	var checksum uint32
	var ipchecksum uint32

	fmt.Printf("natExec src is %s, dest is %s\n", printIPAddr(ipheader.srcAddr), printIPAddr(ipheader.destAddr))

	// プロトコルごとに型を変換
	switch proto {
	case icmp:
		icmpmessage = icmpmessage.ParsePacket(natPacket.packet)
	case udp:
		udpheader = udpheader.ParsePacket(natPacket.packet)
		srcPort = udpheader.srcPort
		destPort = udpheader.destPort
	case tcp:
		tcpheader = tcpheader.ParsePacket(natPacket.packet)
		srcPort = tcpheader.srcPort
		destPort = tcpheader.destPort
	}

	// ICMPだったら、クエリーパケットのみNATする
	if proto == icmp && icmpmessage.icmpHeader.icmpType != ICMP_TYPE_ECHO_REQUEST &&
		icmpmessage.icmpHeader.icmpType != ICMP_TYPE_ECHO_REPLY {
		return nil, fmt.Errorf("ICMPはクエリーパケットのみNATします")
	}

	var entry *natEntry
	if direction == incoming { // NATの外から内への通信時
		if proto == icmp { // ICMPの場合はIDを用いる
			entry = natdevice.natEntry.getNatEntryByGlobal(icmp, ipheader.destAddr, icmpmessage.icmpEcho.identify)
			// NATエントリが登録されていない場合、エラーを返す
			if entry == (&natEntry{}) {
				return nil, fmt.Errorf("No nat entry")
			}
		} else { // UDPとTCPの時はポート番号
			entry = natdevice.natEntry.getNatEntryByGlobal(proto, ipheader.destAddr, destPort)
			// NATエントリが登録されていない場合、エラーを返す
			if entry == (&natEntry{}) {
				return nil, fmt.Errorf("No nat entry")
			}
			fmt.Printf("incoming nat from %s:%d to %s:%d\n",
				printIPAddr(entry.globalIpAddr), entry.globalPort, printIPAddr(entry.localIpAddr), entry.localPort)
		}
		fmt.Printf("incoming ip header src is %s, dest is %s\n",
			printIPAddr(ipheader.srcAddr), printIPAddr(ipheader.destAddr))
		// IPヘッダの送信先アドレスをentryのアドレスにする
		ipheader.destAddr = entry.localIpAddr
		tcpheader.destPort = entry.localPort

	} else { // NATの内から外への通信時
		// ICMPパケット
		//var entry *natEntry
		if proto == icmp {
			entry = natdevice.natEntry.getNatEntryByLocal(icmp, ipheader.srcAddr, icmpmessage.icmpEcho.identify)
		} else {
			entry = natdevice.natEntry.getNatEntryByLocal(proto, ipheader.srcAddr, srcPort)
			fmt.Printf("163: natEntry is %+v\n", entry)
			// dumpNatTables()
		}

		if entry.globalPort == 0 {
			// NATエントリがなかったらエントリ作成
			entry = natdevice.natEntry.createNatEntry(proto)
			if entry == (&natEntry{}) {
				return nil, fmt.Errorf("NAT table is full")
			}
			fmt.Printf("Created new nat table entry global port %d\n", entry.globalPort)
			entry.globalIpAddr = natdevice.outsideIpAddr
			entry.localIpAddr = ipheader.srcAddr
			if proto == icmp {
				entry.localPort = icmpmessage.icmpEcho.identify
			} else {
				if proto == udp {
					entry.localPort = udpheader.srcPort
				} else {
					entry.localPort = tcpheader.srcPort
				}
			}
			fmt.Printf("Now, nat entry local %s:%d to global %s:%d\n",
				printIPAddr(entry.localIpAddr), entry.localPort, printIPAddr(entry.globalIpAddr), entry.globalPort)
		}

		// IPヘッダの送信元アドレスを外側のアドレスにする
		ipheader.srcAddr = entry.globalIpAddr
		tcpheader.srcPort = entry.globalPort
	}

	if proto == icmp {
		checksum = uint32(icmpmessage.icmpHeader.checksum)
		checksum = ^checksum
		checksum -= uint32(icmpmessage.icmpEcho.identify)
		if direction == incoming {
			checksum += uint32(entry.localPort)
		} else {
			checksum += uint32(entry.globalPort)
		}
	} else {
		if proto == udp {
			checksum = uint32(udpheader.checksum)
		} else {
			checksum = uint32(tcpheader.checksum)
			fmt.Printf("before tcp checksum is %x\n", checksum)
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
			fmt.Printf("incoming re calc ipchecsum is %d, %x\n", ipchecksum, uint16(ipchecksum^0xffff))
		} else {
			// sourceのIPアドレスを引く
			checksum -= (entry.localIpAddr - entry.globalIpAddr)
			checksum -= uint32(entry.localPort - entry.globalPort)
			// 桁あふれた1の補数を足し込む
			checksum = (checksum & 0xffff) + checksum>>16
			fmt.Printf("after tcp checksum is %x\n", checksum)
			ipchecksum -= (entry.localIpAddr - entry.globalIpAddr)
		}
	}

	// 計算し直したchecksumをパケットにつけ直す
	if proto == icmp {
		icmpmessage.icmpHeader.checksum = uint16(checksum)
		packet = icmpmessage.ToPacket()
	} else if proto == udp {
		udpheader.checksum = uint16(checksum ^ 0xffff)
		packet = udpheader.ToPacket()
	} else {
		tcpheader.checksum = uint16(checksum ^ 0xffff)
		fmt.Printf("dest port is %d\n", tcpheader.destPort)
		fmt.Printf("1の補数を取ったchecksum is %x\n", tcpheader.checksum)
		fmt.Printf("nat tcp header is %+v\n", tcpheader)
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
	case icmp: // icmpの場合
		for _, v := range entry.icmp {
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
	case icmp: // icmpの場合
		// ICMPのNATテーブルをローカルIPアドレス、ICMPのIDで検索する
		for _, v := range entry.icmp {
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
	case icmp:
		// icmpの場合
		for i, v := range entry.icmp {
			if v == nil {
				// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
				entry.icmp[i] = &natEntry{
					globalPort: uint16(i),
				}
				return entry.icmp[i]
			}
		}
	}
	// 空いているエントリがなかったら
	return &natEntry{}
}
