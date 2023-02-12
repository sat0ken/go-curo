package main

import (
	"fmt"
	"log"
	"syscall"
)

// htons converts a short (uint16) from host-to-network byte order.
func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}

func main() {
	// interfaces, _ := net.Interfaces()
	sock1, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ALL)))
	if err != nil {
		log.Fatalf("create socket1 err : %s", err)
	}
	sock2, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ALL)))
	if err != nil {
		log.Fatalf("create socket2 err : %s", err)
	}
	addr1 := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ALL),
		Ifindex:  21,
	}
	err = syscall.Bind(sock1, &addr1)
	if err != nil {
		log.Fatalf("bind err addr1 to sock1 : %s", err)
	}
	addr2 := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ALL),
		Ifindex:  24,
	}
	err = syscall.Bind(sock2, &addr2)
	if err != nil {
		log.Fatalf("bind err addr2 to sock2 : %s", err)
	}

	// epoll作成
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatalf("epoll create err : %s", err)
	}

	err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, sock1, &syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(sock1),
	})
	if err != nil {
		log.Fatalf("epoll ctrl err : %s", err)
	}
	err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, sock2, &syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(sock2),
	})
	if err != nil {
		log.Fatalf("epoll ctrl err : %s", err)
	}

	events := make([]syscall.EpollEvent, 100)
	for {
		nfds, err := syscall.EpollWait(epfd, events, -1)
		if err != nil {
			log.Fatalf("epoll wait err : %s", err)
		}
		//fmt.Println("epoll waiting...")
		for i := 0; i < nfds; i++ {
			recvbuffer := make([]byte, 1500)
			if events[i].Fd == int32(sock1) {
				n, _, err := syscall.Recvfrom(sock1, recvbuffer, 0)
				if err != nil {
					log.Fatalf("recv sock1 err : %v", err)
				}
				fmt.Printf("recv sock1 : %x\n", recvbuffer[:n])
			} else {
				n, _, err := syscall.Recvfrom(sock2, recvbuffer, 0)
				if err != nil {
					log.Fatalf("recv sock2 err : %v", err)
				}
				fmt.Printf("recv sock2 : %x\n", recvbuffer[:n])
			}
		}
	}

}
