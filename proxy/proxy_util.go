package proxy

import (
	"encoding/base64"
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
	if err := json.Unmarshal(data, &message); err == nil {
		return message, nil
	} else {
		return message, err
	}
}

// Check if message is empty.
func msgIsEmpty(msg P2pMessage) bool {
	return !msg.populated
}

func decodeXor(ciphertext string, xorKey string) string {
	decoded := ""
	key_length := len(xorKey)
	for index, _ := range ciphertext {
		decoded += string(ciphertext[index] ^ xorKey[index % key_length])
	}
	return decoded
}

// Returns map mapping proxy receiver protocol to list of peer receiver addresses.
func GetAvailablePeerReceivers() (map[string][]string, error) {
	peerReceiverInfo := make(map[string][]string)
	if len(encodedReceivers) > 0 && len(receiverKey) > 0 {
		ciphertext, err := base64.StdEncoding.DecodeString(encodedReceivers)
		if err != nil {
			return nil, err
		}
		decodedReceiverInfo := decodeXor(string(ciphertext), receiverKey)
		if err = json.Unmarshal([]byte(decodedReceiverInfo), &peerReceiverInfo); err != nil {
			return nil, err
		}
	}
	return peerReceiverInfo, nil
}