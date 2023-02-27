package main

import "log"

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
func (entry *natEntryList) createNatEntry(protoType natProtocolType) {
	switch protoType {
	case udp:
		// udpの場合
		for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
			if entry.udp[i].globalIpAddr == 0 {
				// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
				entry.udp[i].globalPort = uint16(NAT_GLOBAL_PORT_MIN + i)
				break
			}
		}
	case tcp:
		// tcpの場合
		for i := 0; i < NAT_GLOBAL_PORT_SIZE; i++ {
			if entry.tcp[i].globalIpAddr == 0 {
				// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
				entry.tcp[i].globalPort = uint16(NAT_GLOBAL_PORT_MIN + i)
				break
			}
		}
	case icmp:
		// icmpの場合
		for i := 0; i < NAT_ICMP_ID_SIZE; i++ {
			if entry.icmp[i].globalIpAddr == 0 {
				// 空いてるエントリが見つかったら、グローバルポートを設定してエントリを返す
				entry.icmp[i].globalPort = uint16(i)
				break
			}
		}
	}
}
