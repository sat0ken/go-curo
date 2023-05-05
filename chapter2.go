package main

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

// Global変数でルーティングテーブルを宣言
var iproute radixTreeNode

// Global変数で宣言
var netDeviceList []*netDevice

func runChapter2(mode string) {

	// 直接接続ではないhost2へのルーティングを登録する
	routeEntryTohost2 := ipRouteEntry{
		iptype:  network,
		nexthop: 0xc0a80002,
	}
	// 192.168.2.0/24の経路の登録
	iproute.radixTreeAdd(0xc0a80202&0xffffff00, 24, routeEntryTohost2)

	// epoll作成
	events := make([]syscall.EpollEvent, 10)
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatalf("epoll create err : %s", err)
	}

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
			// socketをepollの監視対象として登録
			err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, sock, &syscall.EpollEvent{
				Events: syscall.EPOLLIN,
				Fd:     int32(sock),
			})
			// ノンブロッキングに設定←epollを使うのでしない
			//err = syscall.SetNonblock(sock, true)
			//if err != nil {
			//	log.Fatalf("set non block is err : %s", err)
			//}
			netaddrs, err := netif.Addrs()
			if err != nil {
				log.Fatalf("get ip addr from nic interface is err : %s", err)
			}

			netdev := netDevice{
				name:     netif.Name,
				macaddr:  setMacAddr(netif.HardwareAddr),
				socket:   sock,
				sockaddr: addr,
				ipdev:    getIPdevice(netaddrs),
			}

			// 直接接続ネットワークの経路をルートテーブルのエントリに設定
			routeEntry := ipRouteEntry{
				iptype: connected,
				netdev: &netdev,
			}
			prefixLen := subnetToPrefixLen(netdev.ipdev.netmask)
			iproute.radixTreeAdd(netdev.ipdev.address&netdev.ipdev.netmask, prefixLen, routeEntry)
			fmt.Printf("Set directly connected route %s/%d via %s\n",
				printIPAddr(netdev.ipdev.address&netdev.ipdev.netmask), prefixLen, netdev.name)

			// netDevice構造体を作成
			// net_deviceの連結リストに連結させる
			netDeviceList = append(netDeviceList, &netdev)
		}
	}

	// 5章で追加
	// chapter5のNW構成で動作させるときは、NATの設定の投入
	if mode == "ch5" {
		configureIPNat("router1-br0", getnetDeviceByName("router1-router2").ipdev.address)
	}

	fmt.Printf("mode is %s start router...\n", mode)

	for {
		// epoll_waitでパケットの受信を待つ
		nfds, err := syscall.EpollWait(epfd, events, -1)
		if err != nil {
			log.Fatalf("epoll wait err : %s", err)
		}
		for i := 0; i < nfds; i++ {
			// デバイスから通信を受信
			for _, netdev := range netDeviceList {
				// イベントがあったソケットとマッチしたらパケットを読み込む処理を実行
				if events[i].Fd == int32(netdev.socket) {
					err := netdev.netDevicePoll(mode)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}
	}
}
