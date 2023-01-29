package main

import (
	"encoding/binary"
	"fmt"
)

const ETHER_TYPE_IP uint16 = 0x0800
const ETHER_TYPE_ARP uint16 = 0x0806
const ETHER_TYPE_IPV6 uint16 = 0x86dd
const ETHERNET_ADDRES_LEN = 6

var ETHERNET_ADDRESS_BROADCAST = [6]uint8{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

type ethernetHeader struct {
	destAddr  [6]uint8 // 宛先MACアドレス
	srcAddr   [6]uint8 // 送信元MACアドレス
	etherType uint16   //イーサタイプ
}

func setMacAddr(macAddrByte []byte) [6]uint8 {
	var macAddrUint8 [6]uint8
	for i, v := range macAddrByte {
		macAddrUint8[i] = v
	}
	return macAddrUint8
}

func setIPAddr(ipAddrByte []byte) [4]uint8 {
	var ipAddrUint8 [4]uint8
	for i, v := range ipAddrByte {
		ipAddrUint8[i] = v
	}
	return ipAddrUint8
}

func byteToUint16(b []byte) uint16 {
	return binary.BigEndian.Uint16(b)
}

func byteToUint32(b []byte) uint32 {
	return binary.BigEndian.Uint32(b)
}

func (netdev netDevice) ethernetInput(packet []byte) {
	// 送られてきた通信をイーサネットのフレームとして解釈する
	netdev.etheHeader.destAddr = setMacAddr(packet[0:6])
	netdev.etheHeader.srcAddr = setMacAddr(packet[6:12])
	netdev.etheHeader.etherType = byteToUint16(packet[12:14])

	// 自分のMACアドレス宛てかブロードキャストの通信かを確認する
	if netdev.macaddr != netdev.etheHeader.destAddr && netdev.etheHeader.destAddr != ETHERNET_ADDRESS_BROADCAST {
		// 自分のMACアドレス宛てかブロードキャストでなければ return する
		return
	}

	// イーサタイプの値から上位プロトコルを特定する
	switch netdev.etheHeader.etherType {
	case ETHER_TYPE_ARP:
		fmt.Println("packet is ARP")
		// Todo: ARPパケットを処理する関数を呼ぶ
		netdev.arpInput(packet[14:])
	case ETHER_TYPE_IP:
		fmt.Println("packet is IP")
		// Todo: IPパケットを処理する関数を呼ぶ
		// netdev.ipInput(packet[14:])
	}

}
