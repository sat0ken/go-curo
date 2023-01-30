package main

import (
	"net"
	"strings"
)

const IP_ADDRESS_LEN = 4

type ipHeader struct {
	headerLen      uint8
	version        uint8
	tos            uint8
	totalLen       uint16
	identify       uint16
	fragOffset     uint16
	ttl            uint8
	protocol       uint8
	headerChecksum uint16
	srcAddr        uint32
	destAddr       uint32
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
		return
	}
}
