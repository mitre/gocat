package agent

import (
	"fmt"

	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/proxy"
)

func (a *Agent) ActivateP2pReceivers() {
	for receiverName, p2pReceiver := range proxy.P2pReceiverChannels {
		if err := p2pReceiver.InitializeReceiver(a.server, a.beaconContact); err != nil {
			output.VerbosePrint(fmt.Sprintf("[*] Initializing p2p receiver %s", receiverName))
			output.VerbosePrint(fmt.Sprintf("[-] Error when initializing p2p receiver %s: %s", receiverName, err.Error()))
		} else {
			p2pReceiver.RunReceiver()
		}
	}
}

func (a *Agent) TerminateP2pReceivers() {
	for receiverName, p2pReceiver := range proxy.P2pReceiverChannels {
		output.VerbosePrint(fmt.Sprintf("[*] Terminating p2p receiver %s", receiverName))
		p2pReceiver.Terminate()
	}
}