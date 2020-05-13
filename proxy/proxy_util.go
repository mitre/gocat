package proxy

import (
	"encoding/json"
)

// Build p2p message and return the bytes of its JSON marshal.
func buildP2pMsgBytes(sourcePaw string, messageType int, payload []byte, srcAddr string) ([]byte, error) {
	p2pMsg := &P2pMessage{
		SourcePaw: sourcePaw,
		SourceAddress: srcAddr,
		MessageType: messageType,
		Payload: payload,
		populated: true,
	}
	return json.Marshal(p2pMsg)
}
// Convert bytes of JSON marshal into P2pMessage struct
func bytesToP2pMsg(data []byte) (P2pMessage, error) {
	var message P2pMessage
	if err := json.Unmarshal(data, &message); err == nil && len(data) > 0 {
		message.populated = true
		return message, nil
	} else {
		return message, err
	}
}

// Check if message is empty.
func msgIsEmpty(msg P2pMessage) bool {
	return !msg.populated
}