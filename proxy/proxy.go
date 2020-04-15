package proxy

import (
    "encoding/json"
    "strings"
    "os"

    "../contact"
)

// Define MessageType values for P2pMessage
const (
	GET_INSTRUCTIONS = 1
	GET_PAYLOAD_BYTES = 2
	SEND_EXECUTION_RESULTS = 3
	RESPONSE_INSTRUCTIONS = 4
	RESPONSE_PAYLOAD_BYTES = 5
	ACK_EXECUTION_RESULTS = 6
)