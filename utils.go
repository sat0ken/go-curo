package main

func byteArraySum(packet []byte) (sum uint) {
	for i, _ := range packet {
		if i%2 == 0 {
			sum += uint(byteToUint16(packet[i:]))
		}
	}
	return sum
}

func calcChecksum(packet []byte) {
	// まず16ビット毎に足す
	packetsum := byteArraySum(packet)
}
