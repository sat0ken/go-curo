package main

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

// htons converts a short (uint16) from host-to-network byte order.
func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}

func main() {
	// epoll作成
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatalf("epoll create err : %s", err)
	}
	events := make([]syscall.EpollEvent, 100)
	var socketarr []int

	interfaces, _ := net.Interfaces()
	for _, netif := range interfaces {
		if netif.Name != "lo" {
			sock, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ALL)))
			if err != nil {
				log.Fatalf("create socket err : %s", err)
			}
			err = syscall.Bind(sock, &syscall.SockaddrLinklayer{
				Protocol: htons(syscall.ETH_P_ALL),
				Ifindex:  netif.Index,
			})
			if err != nil {
				log.Fatalf("bind err addr to socket : %s", err)
			}
			err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, sock, &syscall.EpollEvent{
				Events: syscall.EPOLLIN,
				Fd:     int32(sock),
			})
			if err != nil {
				log.Fatalf("epoll ctrl err : %s", err)
			}
			socketarr = append(socketarr, sock)
		}
	}
	for {
		nfds, err := syscall.EpollWait(epfd, events, -1)
		if err != nil {
			log.Fatalf("epoll wait err : %s", err)
		}
		for i := 0; i < nfds; i++ {
			for _, sock := range socketarr {
				if events[i].Fd == int32(sock) {
					recvbuffer := make([]byte, 1500)
					n, _, err := syscall.Recvfrom(sock, recvbuffer, 0)
					if err != nil {
						log.Fatalf("recv sock err : %v", err)
					}
					fmt.Printf("recv sock : %x\n", recvbuffer[:n])
				}
			}
		}
	}
}
