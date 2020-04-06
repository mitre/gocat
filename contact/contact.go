package contact

import (
	"fmt"
	"path/filepath"
	"reflect"

	//"github.com/mitre/gocat/util"
	//"github.com/mitre/gocat/output"
	"../util"
	"../output"
)

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

// Given requested C2 configuration, returns the requested C2 contact, or the HTTP
// contact upon if error occurs.
func ChooseCommunicationChannel(profile map[string]interface{}, c2Config map[string]string) Contact {
	coms, _ := CommunicationChannels["HTTP"] // Default.
	c2Name, ok := c2Config["c2Name"]
	if ok {
		requestedComs, ok := CommunicationChannels[c2Name]
		if ok {
			if requestedComs.C2RequirementsMet(profile, c2Config) {
				coms = requestedComs
			} else {
				output.VerbosePrint("[-] C2 requirements not met! Defaulting to HTTP")
			}
		} else {
			output.VerbosePrint("[-] Requested C2 channel not found. Defaulting to HTTP")
		}
	} else {
		output.VerbosePrint("[-] Invalid C2 Configuration. c2Name not specified. Defaulting to HTTP")
	}
	return coms
}

// Will download each individual payload listed, write them to disk,
// and will return the full file paths of each downloaded payload.
func DownloadPayloads(profile map[string]interface{}, payloads []interface{}, coms Contact) []string {
	var droppedPayloads []string
	availablePayloads := reflect.ValueOf(payloads)
	for i := 0; i < availablePayloads.Len(); i++ {
		payload := availablePayloads.Index(i).Elem().String()
		location := filepath.Join(payload)
		obtainedPayload := false
		if util.Exists(location) == false {
			output.VerbosePrint(fmt.Sprintf("[*] Fetching new payload bytes: %s", payload))
			payloadBytes := coms.GetPayloadBytes(profile, payload)
			if len(payloadBytes) > 0 {
				util.WritePayloadBytes(location, payloadBytes)
				obtainedPayload = true
			}
		} else {
			obtainedPayload = true
		}
		if obtainedPayload {
			droppedPayloads = append(droppedPayloads, location)
		}
	}
	return droppedPayloads
}