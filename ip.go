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
	address   uint32 // デバイスのIPアドレス
	netmask   uint32 // サブネットマスク
	broadcast uint32 // ブロードキャストアドレス
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

func (ipheader ipHeader) ToPacket() (ipHeaderByte []byte) {
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
	ipHeaderByte = b.Bytes()
	checksum := calcChecksum(ipHeaderByte)

	// checksumをセット
	ipHeaderByte[10] = checksum[0]
	ipHeaderByte[11] = checksum[1]

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
	// 送られてきたバッファをキャストして扱う
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
	fmt.Printf("Received IP packet type %d from %s to %s\n", ipheader.protocol,
		printIPAddr(ipheader.srcAddr), printIPAddr(ipheader.destAddr))

	fmt.Printf("ip header is %+v\n", ipheader)
	// fmt.Printf("input net dev is %s, %d\n", inputdev.name, inputdev.ipdev.address)
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

	// 宛先アドレスがブロードキャストアドレスか自分のIPアドレスの場合
	if ipheader.destAddr == IP_ADDRESS_LIMITED_BROADCAST || inputdev.ipdev.address == ipheader.destAddr {
		// 自分宛の通信として処理
		ipInputToOurs(inputdev, ipheader, packet[20:])
	}
}

/*
自分宛のIPパケットの処理
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/ip.cpp#L26
*/
func ipInputToOurs(inputdev *netDevice, ipheader ipHeader, packet []byte) {
	// 上位プロトコルの処理に移行
	switch ipheader.protocol {
	case IP_PROTOCOL_NUM_ICMP:
		fmt.Println("ICMP received!")
		icmpInput(ipheader.srcAddr, ipheader.destAddr, packet)
	case IP_PROTOCOL_NUM_UDP:
		return
	case IP_PROTOCOL_NUM_TCP:
		return
	default:
		fmt.Printf("Unhandled ip protocol number : %d\n", ipheader.protocol)
		return
	}
}

/*
IPパケットにカプセル化して送信
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/ip.cpp#L102
*/
func ipPacketEncapsulate(destAddr, srcAddr uint32, payload []byte, protocolType uint8) (ipPacket []byte) {
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
	ipPacket = append(ipPacket, ipheader.ToPacket()...)
	// payloadを追加
	ipPacket = append(ipPacket, payload...)

	// Todo: ルートテーブルを検索して送信先IPのMACアドレスがなければ、
	// ARPリクエストを生成して送信して結果を受信してから、ethernetからパケットを送る

	return ipPacket
}
