package agent

import (
	"errors"
	"fmt"

	"github.com/mitre/gocat/contact"
	"github.com/mitre/gocat/output"
)

func (a *Agent) StartTunnel(tunnelConfig *contact.TunnelConfig) error {
	a.usingTunnel = false
	tunnel, ok := contact.CommunicationTunnels[tunnelConfig.Protocol]
	if !ok {
		return errors.New(fmt.Sprintf("Could not find communication tunnel for protocol %s", tunnelConfig.Protocol))
	}
	a.tunnel = tunnel
	if err := a.tunnel.Initialize(tunnelConfig); err != nil {
		return err
	}
	output.VerbosePrint(fmt.Sprintf("[*] Starting %s tunnel", tunnel.GetName()))
	tunnelReady := make(chan bool)
	go a.tunnel.Run(tunnelReady)

	// Wait for tunnel to be ready
	ready := <-tunnelReady
	if ready {
		output.VerbosePrint(fmt.Sprintf("[*] %s tunnel ready and listening on %s.", a.tunnel.GetName(), a.tunnel.GetLocalAddr()))
		a.updateUpstreamDestAddr(a.tunnel.GetLocalAddr())
		a.usingTunnel = true
		return nil
	}
	return errors.New(fmt.Sprintf("Failed to start communication tunnel %s", a.tunnel.GetName()))
}