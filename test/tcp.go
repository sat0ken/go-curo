package main

import (
	"fmt"
	"log"
	"syscall"
)

func main() {
	sock1, _ := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	sock2, _ := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)

	// socketにインターフェイスをbindする
	addr1 := syscall.SockaddrInet4{
		Port: 8080,
		Addr: [4]byte{127, 0, 0, 1},
	}
	addr2 := syscall.SockaddrInet4{
		Port: 8081,
		Addr: [4]byte{127, 0, 0, 1},
	}

	err := syscall.Bind(sock1, &addr1)
	if err != nil {
		log.Fatalf("bind sock1 err : %s", err)
	}
	err = syscall.Bind(sock2, &addr2)
	if err != nil {
		log.Fatalf("bind sock2 err : %s", err)
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

	fmt.Printf("sock1 is %d, sock2 is %d\n", sock1, sock2)

	events := make([]syscall.EpollEvent, 100)
	for {
		nfds, err := syscall.EpollWait(epfd, events, 100)
		if err != nil {
			log.Fatalf("epoll wait err : %s", err)
		}
		// fmt.Printf("epol waiting is %d\n", nfds)

		for i := 0; i < nfds; i++ {
			if events[i].Pad == int32(sock1) {
				accsock1, _, err := syscall.Accept(sock1)
				if err != nil {
					log.Fatalf("sock1 accept err : %s", err)
				}
				fmt.Printf("accept socket1 is %d\n", accsock1)
				err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, accsock1, &syscall.EpollEvent{
					Events: syscall.EPOLLIN | syscall.EPOLLONESHOT,
					Fd:     int32(accsock1),
				})
				if err != nil {
					log.Fatalf("epoll ctrl err : %s", err)
				}

			}
		}
	}
}
