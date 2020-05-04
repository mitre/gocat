package proxy

import (
	"github.com/mitre/gocat/contact"
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

// P2pReceiver defines required functions for relaying messages between peers and an upstream peer/c2.
type P2pReceiver interface {
	InitializeReceiver(server string, upstreamComs contact.Contact) error
	RunReceiver()
	UpdateUpstreamServer(newServer string)
	UpdateUpstreamComs(newComs contact.Contact)
	Terminate()
}

// P2pClient will implement the contact.Contact interface.

// Defines message structure for p2p
type P2pMessage struct {
	SourcePaw string // Paw of agent sending the original request.
	SourceAddress string // return address for responses (e.g. IP + port, pipe path)
	MessageType int
	Payload []byte
	populated bool
}

// P2pReceiverChannels contains the P2pReceiver implementations
var P2pReceiverChannels = map[string]P2pReceiver{}

// Contains the C2 Contact implementations strictly for peer-to-peer communications.
var P2pClientChannels = map[string]contact.Contact{}
