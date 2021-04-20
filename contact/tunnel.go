package contact

// Tunnel defines required functions for providing a comms tunnel between agent and C2.
type Tunnel interface {
	GetName() string
	Initialize(config *TunnelConfig) error
	Run(tunnelReady chan bool) // must be run as a go routine
	GetLocalAddr() string // agent-side address for tunnel
	GetRemoteAddr() string // tunnel destination address
}

type TunnelConfig struct {
	Protocol string // Name of Tunnel protocol
	TunnelAddr string // Address used to connect to or start tunnel
	Username string // Username to authenticate to tunnel
	Password string // Password to authenticate to tunnel
	TunnelDest string // Address that tunnel will ultimately connect to
}

// CommunicationTunnels contains available Tunnel implementations
var CommunicationTunnels = map[string]Tunnel{}

func GetAvailableCommTunnels() []string {
	tunnelNames := make([]string, 0, len(CommunicationTunnels))
	for name := range CommunicationTunnels {
		tunnelNames = append(tunnelNames, name)
	}
	return tunnelNames
}

func BuildTunnelConfig(protocol, tunnelAddr, destAddr, user, password string) *TunnelConfig {
	return &TunnelConfig{
		Protocol: protocol,
		TunnelAddr: destAddr,
		Username: user,
		Password: password,
		TunnelDest: destAddr,
	}
}