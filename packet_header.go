package main

import (
	"bytes"
	"encoding/binary"
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
}

func (udpheder *udpHeader) ToPacket() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, udpheder)

	return buf.Bytes()
}

func (tcpheader *tcpHeader) ToPacket() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, tcpheader)

	return buf.Bytes()
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
	return tcpHeader{
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
}
