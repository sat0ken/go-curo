package main

import (
	"fmt"
	"log"
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
	packet interface{}
}
type natEntry struct {
	globalIpAddr uint32
	localIpAddr  uint32
	globalPort   uint16
	localPort    uint16
}

// ICMP, UDP, TCPのNATテーブルのセット
type natEntryList struct {
	icmp []natEntry
	tcp  []natEntry
	udp  []natEntry
}

// NATの内側のip_deviceが持つNATデバイス
type natDevice struct {
	outsideIpAddr uint32
	natEntry      *natEntryList
}

func configureIPNat(inside netDevice, outside netDevice) {
	if inside == (netDevice{}) || outside == (netDevice{}) ||
		inside.ipdev == (ipDevice{}) || outside.ipdev == (ipDevice{}) {
		log.Fatal("Failed to configure NAT")
	}
	// 内から外へ出るときに変換するIPアドレス設定
	inside.ipdev.natdev.outsideIpAddr = outside.ipdev.address
}

func dumpNatTables() {
	fmt.Println("|-PROTO-|---------LOCAL---------|--------GLOBAL---------|")
	for _, netdev := range netDeviceList {
		if netdev.ipdev != (ipDevice{}) && netdev.ipdev.natdev != (natDevice{}) {
			for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
				if netdev.ipdev.natdev.natEntry.tcp[i].globalPort != 0 {
					fmt.Printf("|  TCP  | %15s:%05d | %15s:%05d |\n",
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
func execNat(ipheader ipHeader, natPacket natPacketHeader, natdevice natDevice, proto natProtocolType, direction natDirectionType) error {
	var icmpmessage icmpMessage
	var udpheader udpHeader
	var tcpheader tcpHeader
	var srcPort, destPort uint16

	// プロトコルごとに型を変換
	switch proto {
	case icmp:
		icmpmessage = natPacket.packet.(icmpMessage)
	case udp:
		udpheader = natPacket.packet.(udpHeader)
		srcPort = udpheader.srcPort
		destPort = udpheader.destPort
	case tcp:
		tcpheader = natPacket.packet.(tcpHeader)
		srcPort = tcpheader.srcPort
		destPort = tcpheader.destPort
	}

	// ICMPだったら、クエリーパケットのみNATする
	if proto == icmp && icmpmessage.icmpHeader.icmpType != ICMP_TYPE_ECHO_REQUEST &&
		icmpmessage.icmpHeader.icmpType != ICMP_TYPE_ECHO_REPLY {
		return fmt.Errorf("ICMPはクエリーパケットのみNATします")
	}

	var entry natEntry
	if direction == incoming { // NATの外から内への通信時
		if proto == icmp { // ICMPの場合はIDを用いる
			entry = natdevice.natEntry.getNatEntryByGlobal(icmp, ipheader.destAddr, icmpmessage.icmpEcho.identify)
		} else { // UDPとTCPの時はポート番号
			entry = natdevice.natEntry.getNatEntryByLocal(proto, ipheader.destAddr, destPort)
		}
		// NATエントリが登録されていない場合、エラーを返す
		if entry == (natEntry{}) {
			return fmt.Errorf("No nat entry")
		}
	} else { // NATの内から外への通信時
		// ICMPパケット
		if proto == icmp {
			entry = natdevice.natEntry.getNatEntryByLocal(icmp, ipheader.srcAddr, icmpmessage.icmpEcho.identify)
		} else {
			entry = natdevice.natEntry.getNatEntryByLocal(proto, ipheader.srcAddr, srcPort)
		}
		if entry == (natEntry{}) {
			// NATエントリがなかったらエントリ作成
			entry = natdevice.natEntry.createNatEntry(proto)
		}
	}

	var checksum uint32
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
		}
		checksum = ^checksum
		// checksumの差分の計算
		if direction == incoming {
			checksum -= ipheader.destAddr & 0xffff
			checksum -= ipheader.destAddr >> 16
			checksum -= uint32(destPort)
			checksum += entry.localIpAddr & 0xffff
			checksum += entry.localIpAddr >> 16
			checksum += uint32(entry.localPort)
		} else {
			checksum -= ipheader.srcAddr & 0xffff
			checksum -= ipheader.srcAddr >> 16
			checksum -= uint32(srcPort)
			checksum += natdevice.outsideIpAddr & 0xffff
			checksum += natdevice.outsideIpAddr >> 16
			checksum += uint32(entry.globalPort)
		}
	}
	checksum = ^checksum
	if checksum > 0xffff {
		checksum = (checksum & 0xffff) + (checksum >> 16)
	}

	// 計算し直したchecksumをパケットにつけ直す
	if proto == icmp {
		icmpmessage.icmpHeader.checksum = uint16(checksum)
	} else if proto == udp {
		udpheader.checksum = uint16(checksum)
	} else {
		tcpheader.checksum = uint16(checksum)
	}
	return nil
}

/*
グローバルアドレスとグローバルポートからNATエントリを取得
*/
func (entry *natEntryList) getNatEntryByGlobal(protoType natProtocolType, ipaddr uint32, port uint16) natEntry {

	switch protoType {
	case udp: // udpの場合
		if entry.udp[port-NAT_GLOBAL_PORT_MIN].globalIpAddr == ipaddr &&
			entry.udp[port-NAT_GLOBAL_PORT_MIN].globalPort == port {
			return entry.udp[port-NAT_GLOBAL_PORT_MIN]
		}
	case tcp: // tcpの場合
		if entry.tcp[port-NAT_GLOBAL_PORT_MIN].globalIpAddr == ipaddr &&
			entry.tcp[port-NAT_GLOBAL_PORT_MIN].globalPort == port {
			return entry.tcp[port-NAT_GLOBAL_PORT_MIN]
		}
	case icmp: // icmpの場合
		if entry.icmp[port].globalIpAddr == ipaddr &&
			entry.icmp[port].globalPort == port {
			return entry.icmp[port]
		}
	}
	return natEntry{}
}

/*
ローカルアドレスとローカルポートからNATエントリを取得
*/
func (entry *natEntryList) getNatEntryByLocal(protoType natProtocolType, ipaddr uint32, port uint16) natEntry {

	switch protoType {
	case udp: // udpの場合
		// UDPのNATテーブルをローカルIPアドレス, ローカルポートで検索する
		for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
			if entry.udp[i].localIpAddr == ipaddr &&
				entry.udp[i].localPort == port {
				return entry.udp[i]
			}
		}
	case tcp: // tcpの場合
		// TCPのNATテーブルをローカルIPアドレス, ローカルポートで検索する
		for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
			if entry.tcp[i].localIpAddr == ipaddr &&
				entry.tcp[i].localPort == port {
				return entry.tcp[i]
			}
		}
	case icmp: // icmpの場合
		// ICMPのNATテーブルをローカルIPアドレス、ICMPのIDで検索する
		for i := 0; i < NAT_ICMP_ID_SIZE; i++ {
			if entry.icmp[i].localIpAddr == ipaddr &&
				entry.icmp[i].localPort == port {
				return entry.icmp[i]
			}
		}
	}
	// テーブルに一致するエントリがなかったらnullを返す
	return natEntry{}
}

/*
空いてるポートを探し、NATエントリを作成する
*/
func (entry *natEntryList) createNatEntry(protoType natProtocolType) natEntry {
	switch protoType {
	case udp:
		// udpの場合
		for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
			if entry.udp[i].globalIpAddr == 0 {
				// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
				entry.udp[i].globalPort = uint16(NAT_GLOBAL_PORT_MIN + i)
				return entry.udp[i]
			}
		}
	case tcp:
		// tcpの場合
		for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
			if entry.tcp[i].globalIpAddr == 0 {
				// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
				entry.tcp[i].globalPort = uint16(NAT_GLOBAL_PORT_MIN + i)
				return entry.tcp[i]
			}
		}
	case icmp:
		// icmpの場合
		for i := 0; i < NAT_ICMP_ID_SIZE; i++ {
			if entry.icmp[i].globalIpAddr == 0 {
				// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
				entry.icmp[i].globalPort = uint16(i)
				return entry.icmp[i]
			}
		}
	}
	// 空いているエントリがなかったら
	return natEntry{}
}
