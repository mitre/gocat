package agent

import (
	"fmt"

	"github.com/mitre/gocat/contact"
	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/proxy"
)

func (a *Agent) ActivateP2pReceivers() {
	a.validP2pReceivers = make(map[string]contact.Contact{})
	for receiverName, p2pReceiver := range proxy.P2pReceiverChannels {
		if err := p2pReceiver.InitializeReceiver(a.server, a.beaconContact); err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] Error when initializing p2p receiver %s: %s", receiverName, err.Error()))
		} else {
			output.VerbosePrint(fmt.Sprintf("[*] Initialized p2p receiver %s", receiverName))
			a.validP2pReceivers[receiverName] = p2pReceiver
			p2pReceiver.RunReceiver()
		}
	}
}

func (a *Agent) TerminateP2pReceivers() {
	for receiverName, p2pReceiver := range a.validP2pReceivers {
		output.VerbosePrint(fmt.Sprintf("[*] Terminating p2p receiver %s", receiverName))
		p2pReceiver.Terminate()
	}
}