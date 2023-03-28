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
