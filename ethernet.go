package main

import (
	"bytes"
	"log"
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

func (ethHeader ethernetHeader) ToPacket() []byte {
	var b bytes.Buffer
	b.Write(macToByte(ethHeader.destAddr))
	b.Write(macToByte(ethHeader.srcAddr))
	b.Write(uint16ToByte(ethHeader.etherType))
	return b.Bytes()
}

func setMacAddr(macAddrByte []byte) [6]uint8 {
	var macAddrUint8 [6]uint8
	for i, v := range macAddrByte {
		macAddrUint8[i] = v
	}
	return macAddrUint8
}

func macToByte(macaddr [6]uint8) (b []byte) {
	for _, v := range macaddr {
		b = append(b, v)
	}
	return b
}

// イーサネットの受信処理
func ethernetInput(netdev *netDevice, packet []byte) {
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
		arpInput(netdev, packet[14:])
	case ETHER_TYPE_IP:
		ipInput(netdev, packet[14:])
	}
}

// イーサネットにカプセル化して送信
func ethernetOutput(netdev *netDevice, destaddr [6]uint8, packet []byte, ethType uint16) {
	// イーサネットヘッダのパケットを作成
	ethHeaderPacket := ethernetHeader{
		destAddr:  destaddr,
		srcAddr:   netdev.macaddr,
		etherType: ethType,
	}.ToPacket()
	// イーサネットヘッダに送信するパケットをつなげる
	ethHeaderPacket = append(ethHeaderPacket, packet...)
	// ネットワークデバイスに送信する
	err := netdev.netDeviceTransmit(ethHeaderPacket)
	if err != nil {
		log.Fatalf("netDeviceTransmit is err : %v", err)
	}
}
