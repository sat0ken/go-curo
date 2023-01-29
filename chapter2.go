package main

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

func runChapter2() {
	var netDeviceList []netDevice

	// ネットワークインターフェイスの情報を取得
	interfaces, _ := net.Interfaces()
	for _, netif := range interfaces {
		// 無視するインターフェイスか確認
		if !isIgnoreInterfaces(netif.Name) {
			// socketをオープン
			sock, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ALL)))
			if err != nil {
				log.Fatalf("create socket err : %s", err)
			}
			// socketにインターフェイスをbindする
			addr := syscall.SockaddrLinklayer{
				Protocol: htons(syscall.ETH_P_ALL),
				Ifindex:  netif.Index,
			}
			err = syscall.Bind(sock, &addr)
			if err != nil {
				log.Fatalf("bind err : %s", err)
			}
			fmt.Printf("Created device %s socket %d adddress %s\n",
				netif.Name, sock, netif.HardwareAddr.String())
			// ノンブロッキングに設定
			err = syscall.SetNonblock(sock, true)
			if err != nil {
				log.Fatalf("set non block is err : %s", err)
			}
			netaddrs, err := netif.Addrs()
			if err != nil {
				log.Fatalf("get ip addr from nic interface is err : %s", err)
			}

			// netDevice構造体を作成
			// net_deviceの連結リストに連結させる
			netDeviceList = append(netDeviceList, netDevice{
				name:     netif.Name,
				macaddr:  setMacAddr(netif.HardwareAddr),
				socket:   sock,
				sockaddr: addr,
				ipdev:    getIPdevice(netaddrs),
			})
		}
	}

	for {
		// デバイスから通信を受信
		for _, netdev := range netDeviceList {
			err := netdev.netDevicePoll("chapter2")
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
