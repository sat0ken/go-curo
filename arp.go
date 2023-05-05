package main

import (
	"bytes"
	"fmt"
)

const ARP_OPERATION_CODE_REQUEST = 1
const ARP_OPERATION_CODE_REPLY = 2
const ARP_HTYPE_ETHERNET uint16 = 0001

/**
 * ARPテーブル
 * グローバル変数にテーブルを保持
 */
var ArpTableEntryList []arpTableEntry

type arpIPToEthernet struct {
	hardwareType        uint16   // ハードウェアタイプ
	protocolType        uint16   // プロトコルタイプ
	hardwareLen         uint8    // ハードウェアアドレス長
	protocolLen         uint8    // プロトコルアドレス長
	opcode              uint16   // オペレーションコード
	senderHardwareAddr  [6]uint8 // 送信元のMACアドレス
	senderIPAddr        uint32   // 送信者のIPアドレス
	targetHardwareAddrr [6]uint8 // ターゲットのMACアドレス
	targetIPAddr        uint32   // ターゲットのIPアドレス
}

type arpTableEntry struct {
	macAddr [6]uint8
	ipAddr  uint32
	netdev  *netDevice
}

func (arpmsg arpIPToEthernet) ToPacket() []byte {
	var b bytes.Buffer

	b.Write(uint16ToByte(arpmsg.hardwareType))
	b.Write(uint16ToByte(arpmsg.protocolType))
	b.Write([]byte{arpmsg.hardwareLen})
	b.Write([]byte{arpmsg.protocolLen})
	b.Write(uint16ToByte(arpmsg.opcode))
	b.Write(macToByte(arpmsg.senderHardwareAddr))
	b.Write(uint32ToByte(arpmsg.senderIPAddr))
	b.Write(macToByte(arpmsg.targetHardwareAddrr))
	b.Write(uint32ToByte(arpmsg.targetIPAddr))

	return b.Bytes()
}

/*
ARPパケットの受信処理
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/arp.cpp#L139
*/
func arpInput(netdev *netDevice, packet []byte) {
	// ARPパケットの規定より短かったら
	if len(packet) < 28 {
		fmt.Printf("received ARP Packet is too short")
		return
	}

	// 構造体にセット
	arpMsg := arpIPToEthernet{
		hardwareType:        byteToUint16(packet[0:2]),
		protocolType:        byteToUint16(packet[2:4]),
		hardwareLen:         packet[4],
		protocolLen:         packet[5],
		opcode:              byteToUint16(packet[6:8]),
		senderHardwareAddr:  setMacAddr(packet[8:14]),
		senderIPAddr:        byteToUint32(packet[14:18]),
		targetHardwareAddrr: setMacAddr(packet[18:24]),
		targetIPAddr:        byteToUint32(packet[24:28]),
	}

	switch arpMsg.protocolType {
	case ETHER_TYPE_IP:

		if arpMsg.hardwareLen != ETHERNET_ADDRES_LEN {
			fmt.Println("Illegal hardware address length")
			return
		}

		if arpMsg.protocolLen != IP_ADDRESS_LEN {
			fmt.Println("Illegal protocol address length")
			return
		}

		// オペレーションコードによって分岐
		if arpMsg.opcode == ARP_OPERATION_CODE_REQUEST {
			// ARPリクエストの受信
			fmt.Printf("ARP Request Packet is %+v\n", arpMsg)
			arpRequestArrives(netdev, arpMsg)
		} else {
			// ARPリプライの受信
			fmt.Printf("ARP Reply Packet is %+v\n", arpMsg)
			arpReplyArrives(netdev, arpMsg)
		}
	}
}

/*
ARPテーブルにエントリの追加と更新
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/arp.cpp#L23
*/
func addArpTableEntry(netdev *netDevice, ipaddr uint32, macaddr [6]uint8) {

	// 既存のARPテーブルの更新が必要か確認
	if len(ArpTableEntryList) != 0 {
		for _, arpTable := range ArpTableEntryList {
			// IPアドレスは同じだがMacアドレスが異なる場合は更新
			if arpTable.ipAddr == ipaddr && arpTable.macAddr != macaddr {
				arpTable.macAddr = macaddr
			}
			// Macアドレスは同じだがIPアドレスが変わった場合は更新
			if arpTable.macAddr == macaddr && arpTable.ipAddr != ipaddr {
				arpTable.ipAddr = ipaddr
			}
			// 既に存在する場合はreturnする
			if arpTable.macAddr == macaddr && arpTable.ipAddr == ipaddr {
				return
			}
		}
	}

	ArpTableEntryList = append(ArpTableEntryList, arpTableEntry{
		macAddr: macaddr,
		ipAddr:  ipaddr,
		netdev:  netdev,
	})
	//fmt.Printf("ARP TABEL is %+v\n", ArpTableEntryList)
}

/*
ARPテーブルの検索
*/
func searchArpTableEntry(ipaddr uint32) ([6]uint8, *netDevice) {
	if len(ArpTableEntryList) != 0 {
		for _, arpTable := range ArpTableEntryList {
			if arpTable.ipAddr == ipaddr {
				return arpTable.macAddr, arpTable.netdev
			}
		}
	}
	return [6]uint8{}, nil
}

/*
ARPリクエストパケットの受信処理
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/arp.cpp#L181
*/
func arpRequestArrives(netdev *netDevice, arp arpIPToEthernet) {
	// IPアドレスが設定されているデバイスからの受信かつ要求されているアドレスが自分の物だったら
	if netdev.ipdev.address != 00000000 && netdev.ipdev.address == arp.targetIPAddr {
		fmt.Printf("Sending arp reply to %s\n", printIPAddr(arp.targetIPAddr))
		// APRリプライのパケットを作成
		arpPacket := arpIPToEthernet{
			hardwareType:        ARP_HTYPE_ETHERNET,
			protocolType:        ETHER_TYPE_IP,
			hardwareLen:         ETHERNET_ADDRES_LEN,
			protocolLen:         IP_ADDRESS_LEN,
			opcode:              ARP_OPERATION_CODE_REPLY,
			senderHardwareAddr:  netdev.macaddr,
			senderIPAddr:        netdev.ipdev.address,
			targetHardwareAddrr: arp.senderHardwareAddr,
			targetIPAddr:        arp.senderIPAddr,
		}.ToPacket()

		// ethernetでカプセル化して送信
		ethernetOutput(netdev, arp.senderHardwareAddr, arpPacket, ETHER_TYPE_ARP)
	}
}

/*
ARPリプライパケットの受信処理
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/arp.cpp#L213
*/
func arpReplyArrives(netdev *netDevice, arp arpIPToEthernet) {
	// IPアドレスが設定されているデバイスからの受信だったら
	if netdev.ipdev.address != 00000000 {
		fmt.Printf("Added arp table entry by arp reply (%s => %s)\n", printIPAddr(arp.senderIPAddr), printMacAddr(arp.senderHardwareAddr))
		// ARPテーブルエントリの追加
		addArpTableEntry(netdev, arp.senderIPAddr, arp.senderHardwareAddr)
	}
}

/*
ARPリクエストの送信
https://github.com/kametan0730/interface_2022_11/blob/master/chapter2/arp.cpp#L111
*/
func sendArpRequest(netdev *netDevice, targetip uint32) {
	fmt.Printf("Sending arp request via %s for %x\n", netdev.name, targetip)
	// APRリクエストのパケットを作成
	arpPacket := arpIPToEthernet{
		hardwareType:        ARP_HTYPE_ETHERNET,
		protocolType:        ETHER_TYPE_IP,
		hardwareLen:         ETHERNET_ADDRES_LEN,
		protocolLen:         IP_ADDRESS_LEN,
		opcode:              ARP_OPERATION_CODE_REQUEST,
		senderHardwareAddr:  netdev.macaddr,
		senderIPAddr:        netdev.ipdev.address,
		targetHardwareAddrr: ETHERNET_ADDRESS_BROADCAST,
		targetIPAddr:        targetip,
	}.ToPacket()
	// ethernetでカプセル化して送信
	ethernetOutput(netdev, ETHERNET_ADDRESS_BROADCAST, arpPacket, ETHER_TYPE_ARP)
}
