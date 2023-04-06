package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type udpHeader struct {
	srcPort  uint16
	destPort uint16
	length   uint16
	checksum uint16
}

type tcpHeader struct {
	srcPort    uint16
	destPort   uint16
	seq        uint32
	ackseq     uint32
	offset     uint8
	tcpflag    uint8
	window     uint16
	checksum   uint16
	urgPointer uint16
	options    []byte
	tcpdata    []byte
}

type dummyHeader struct {
	srcAddr  uint32 // 送信元IPアドレス
	destAddr uint32 // 送信先IPアドレス
	protocol uint16
	length   uint16
}

func (dummyHeader *dummyHeader) ToPacket() []byte {
	var b bytes.Buffer

	b.Write(uint32ToByte(dummyHeader.srcAddr))
	b.Write(uint32ToByte(dummyHeader.destAddr))
	b.Write(uint16ToByte(dummyHeader.protocol))
	b.Write(uint16ToByte(dummyHeader.length))
	return b.Bytes()
}

func (udpheder *udpHeader) ToPacket() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, udpheder)

	return buf.Bytes()
}

func (tcpheader *tcpHeader) ToPacket() []byte {
	var b bytes.Buffer

	b.Write(uint16ToByte(tcpheader.srcPort))
	b.Write(uint16ToByte(tcpheader.destPort))
	b.Write(uint32ToByte(tcpheader.seq))
	b.Write(uint32ToByte(tcpheader.ackseq))
	b.Write([]byte{tcpheader.offset})
	b.Write([]byte{tcpheader.tcpflag})
	b.Write(uint16ToByte(tcpheader.window))
	b.Write(uint16ToByte(tcpheader.checksum))
	b.Write(uint16ToByte(tcpheader.urgPointer))

	// TCPオプションがあれば
	if len(tcpheader.options) != 0 {
		b.Write(tcpheader.options)
	}
	// TCPデータがあれば
	if len(tcpheader.tcpdata) != 0 {
		b.Write(tcpheader.tcpdata)
	}

	return b.Bytes()
}

func (udpheader *udpHeader) ParsePacket(packet []byte) udpHeader {
	return udpHeader{
		srcPort:  byteToUint16(packet[0:2]),
		destPort: byteToUint16(packet[2:4]),
		length:   byteToUint16(packet[4:6]),
		checksum: byteToUint16(packet[6:8]),
	}
}

func (tcpheader *tcpHeader) ParsePacket(packet []byte) tcpHeader {
	header := tcpHeader{
		srcPort:    byteToUint16(packet[0:2]),
		destPort:   byteToUint16(packet[2:4]),
		seq:        byteToUint32(packet[4:8]),
		ackseq:     byteToUint32(packet[8:12]),
		offset:     packet[12],
		tcpflag:    packet[13],
		window:     byteToUint16(packet[14:16]),
		checksum:   byteToUint16(packet[16:18]),
		urgPointer: byteToUint16(packet[18:20]),
	}
	headerLength := header.offset >> 2
	// TCPのオプションがあれば
	if 20 < headerLength {
		header.options = packet[20:headerLength]
	}

	// TCPデータがあれば
	if int(headerLength) < len(packet) {
		header.tcpdata = packet[headerLength:]
	}
	fmt.Printf("parsed tcp header is %+v\n", header)
	return header
}
