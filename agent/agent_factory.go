package agent

import (
	"github.com/mitre/gocat/contact"
)

// Creates and initializes a new Agent. Upon success, returns a pointer to the agent and nil Error.
// Upon failure, returns nil and an error.
func AgentFactory(contactConfig *contact.ContactConfig, group string, enableLocalP2pReceivers bool, initialDelay int, paw string, originLinkID int) (*Agent, error) {
	newAgent := &Agent{}
	if err := newAgent.Initialize(contactConfig, group, enableLocalP2pReceivers, initialDelay, paw, originLinkID); err != nil {
		return nil, err
	} else {
		newAgent.Sleep(newAgent.initialDelay)
		return newAgent, nil
	}
}

func (a *Agent) setInitialPeerInfo(contactConfig *contact.ContactConfig) {
	contactProtocol = contactConfig.Protocol
	c2Addr = contactConfig.ServerAddr
	a.exhaustedPeerReceivers = make(map[string][]string)
	a.usingPeerReceivers = false
	a.availablePeerReceivers, err = proxy.GetAvailablePeerReceivers()
	a.availablePeerReceivers[contactProtocol] = append(a.availablePeerReceivers[contactProtocol], c2Addr)
	if err != nil {
		return err
	}
	a.DiscoverPeers()
}