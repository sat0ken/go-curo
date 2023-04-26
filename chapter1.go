package main

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

func runChapter1() {
	var netDeviceList []netDevice
	events := make([]syscall.EpollEvent, 10)

	// epoll作成
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
			if err != nil {
				log.Fatalf("epoll ctrl err : %s", err)
			}
			// ノンブロッキングに設定
			//err = syscall.SetNonblock(sock, true)
			//if err != nil {
			//	log.Fatalf("set non block is err : %s", err)
			//}
			// netDevice構造体を作成
			// net_deviceの連結リストに連結させる
			netDeviceList = append(netDeviceList, netDevice{
				name:     netif.Name,
				macaddr:  setMacAddr(netif.HardwareAddr),
				socket:   sock,
				sockaddr: addr,
			})
		}
	}

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
					err = netdev.netDevicePoll("ch1")
					if err != nil {
						log.Fatal(err)
					}
				}

			}
		}

	}
}
