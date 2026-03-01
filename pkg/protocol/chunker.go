package protocol

// BLEMaxPacketSize is the maximum BLE packet size for GATT writes.
// Standard BLE 4.x uses 20-byte ATT payloads; BLE 5.x can negotiate larger MTUs.
const BLEMaxPacketSize = 20

// ChunkForBLE splits a message into BLE-sized packets.
// Each packet is at most maxSize bytes (default BLEMaxPacketSize).
func ChunkForBLE(msg []byte, maxSize int) [][]byte {
	if maxSize <= 0 {
		maxSize = BLEMaxPacketSize
	}
	if len(msg) == 0 {
		return nil
	}

	numChunks := (len(msg) + maxSize - 1) / maxSize
	chunks := make([][]byte, 0, numChunks)

	for offset := 0; offset < len(msg); offset += maxSize {
		end := offset + maxSize
		if end > len(msg) {
			end = len(msg)
		}
		chunk := make([]byte, end-offset)
		copy(chunk, msg[offset:end])
		chunks = append(chunks, chunk)
	}

	return chunks
}

// Reassemble combines BLE packets back into a complete message.
// It expects chunks in order and concatenates them.
func Reassemble(chunks [][]byte) []byte {
	totalLen := 0
	for _, c := range chunks {
		totalLen += len(c)
	}
	msg := make([]byte, 0, totalLen)
	for _, c := range chunks {
		msg = append(msg, c...)
	}
	return msg
}
