package contact

const (
	ok = 200
	created = 201
)

//Contact defines required functions for communicating with the server
type Contact interface {
	GetBeaconBytes(profile map[string]interface{}) []byte
	GetPayloadBytes(profile map[string]interface{}, payload string) []byte
	C2RequirementsMet(profile map[string]interface{}, criteria map[string]string) bool
	SendExecutionResults(profile map[string]interface{}, result map[string]interface{})
}

//CommunicationChannels contains the contact implementations
var CommunicationChannels = map[string]Contact{}