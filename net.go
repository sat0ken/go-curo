package main

import (
	"fmt"
	"syscall"
)

var IGNORE_INTERFACES = []string{"lo", "bond0", "dummy0", "tunl0", "sit0"}

type netDevice struct {
	name       string
	macaddr    [6]uint8
	socket     int
	sockaddr   syscall.SockaddrLinklayer
	etheHeader ethernetHeader
	ipdev      ipDevice
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
func (netdev netDevice) netDevicePoll(mode string) error {
	recvbuffer := make([]byte, 1500)
	n, _, err := syscall.Recvfrom(netdev.socket, recvbuffer, 0)
	if err != nil {
		if n == -1 {
			return nil
		} else {
			return fmt.Errorf("recv err, n is %d, device is %s, err is %s", n, netdev.name, err)
		}
	}

	// Chapter1では受信したパケットをprintするだけ
	if mode == "chapter1" {
		fmt.Printf("Received %d bytes from %s: %x\n", n, netdev.name, recvbuffer[:n])
	} else {
		ethernetInput(&netdev, recvbuffer[:n])
	}

	return nil
}

// インターフェイス名からデバイスを探す
func getnetDeviceByName(devlist []netDevice, name string) netDevice {
	for _, dev := range devlist {
		if name == dev.name {
			return dev
		}
	}
	return netDevice{}
}
