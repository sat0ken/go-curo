package main

import (
	"fmt"
	"net"
	"strings"
)

const IP_ADDRESS_LEN = 4
const IP_ADDRESS_LIMITED_BROADCAST uint32 = 0xffffffff

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

type ipDevice struct {
	address   uint32 // デバイスのIPアドレス
	netmask   uint32 // サブネットマスク
	broadcast uint32 // ブロードキャストアドレス
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

func toIPString(ip uint32) string {
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
		toIPString(ipheader.srcAddr), toIPString(ipheader.destAddr))

	fmt.Printf("ip header is %+v\n", ipheader)
	fmt.Printf("input net dev is %s, %d\n", inputdev.name, inputdev.ipdev.address)
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

	// 宛先アドレスがブロードキャストアドレスの場合
	if ipheader.destAddr == IP_ADDRESS_LIMITED_BROADCAST && inputdev.ipdev.address == ipheader.destAddr {
		// 自分宛の通信として処理
		// ipInputToOurs()
		fmt.Println("Todo: 自分宛の通信として処理")
	}

	// 宛先IPアドレスをルータが持ってるか調べる

}
