package gocuro

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

var IGNORE_INTERFACES = []string{"lo", "bond0", "dummy0", "tunl0", "sit0"}

type netDevice struct {
	name       string
	macaddr    [6]uint8
	socket     int
	sockaddr   syscall.SockaddrLinklayer
	etheHeader ethernetHeader
}

func isIgnoreInterfaces(name string) bool {
	for _, v := range IGNORE_INTERFACES {
		if v == name {
			return true
		}
	}
	return false
}

// htons converts a short (uint16) from host-to-network byte order.
func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}

// ネットデバイスの送信処理
func (netdev netDevice) netDeviceTransmit(data []byte) error {
	err := syscall.Sendto(netdev.socket, data, 0, &netdev.sockaddr)
	if err != nil {
		return err
	}
	return nil
}

// ネットデバイスの受信処理
func (netdev netDevice) netDevicePoll() error {
	recvbuffer := make([]byte, 1500)
	n, _, err := syscall.Recvfrom(netdev.socket, recvbuffer, 0)
	if err != nil {
		if n == -1 {
			return nil
		} else {
			return fmt.Errorf("recv err, n is %d, device is %s, err is %s", n, netdev.name, err)
		}
	}
	fmt.Printf("Received %d bytes from %s: %x\n", n, netdev.name, recvbuffer[:n])
	return nil
}

func Chapter1() {
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
		// デバイスから通信を受信
		for _, netdev := range netDeviceList {
			err := netdev.netDevicePoll()
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
