package gocuro

type ethernetHeader struct {
	destAddr  [6]uint8
	srcAddr   [6]uint8
	etherType uint16
}

func (netdev netDevice) ethernetInput(packet []byte) {

}
