package contact

const (
	ok = 200
	created = 201
)

//Contact defines required functions for communicating with the server
type Contact interface {
	GetBeaconBytes(profile map[string]interface{}) []byte
	GetPayloadBytes(profile map[string]interface{}, payload string) ([]byte, string)
	C2RequirementsMet(profile map[string]interface{}, c2Config map[string]string) (bool, map[string]string)
	SendExecutionResults(profile map[string]interface{}, result map[string]interface{})
	GetName() string
	SetUpstreamDestAddr(upstreamDestAddr string)
	UploadFileBytes(profile map[string]interface{}, uploadName string, data []byte) error
}

type ContactConfig struct {
	Protocol string // name of C2 protocol
	ServerAddr string // address of C2 server
	UpstreamDestAddr string // address of server/peer that agent uses to contact C2
	Key string // C2 key used to authenticate
	HttpProxyGateway string
	TunnelConfig *TunnelConfig
}

//CommunicationChannels contains the contact implementations
var CommunicationChannels = map[string]Contact{}

func GetAvailableCommChannels() []string {
	channels := make([]string, 0, len(CommunicationChannels))
	for k := range CommunicationChannels {
		channels = append(channels, k)
	}
	return channels
}

func BuildContactConfig(server, protocol, key, httpProxyGateway string, tunnelConfig *TunnelConfig) *ContactConfig {
	return &ContactConfig{
		Protocol: protocol,
		ServerAddr: server,
		UpstreamDestAddr: server,
		Key: key,
		HttpProxyGateway: httpProxyGateway,
		TunnelConfig: tunnelConfig,
	}
}