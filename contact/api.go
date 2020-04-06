package contact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	//"github.com/mitre/gocat/output"
	//"github.com/mitre/gocat/util"
	"../output"
	"../util"
)

var (
	apiBeacon = "/beacon"
)

//API communicates through HTTP
type API struct { }

func init() {
	CommunicationChannels["HTTP"] = API{}
}

//GetInstructions sends a beacon and returns response.
func (contact API) GetBeaconBytes(profile map[string]interface{}) []byte {
	data, _ := json.Marshal(profile)
	address := fmt.Sprintf("%s%s", profile["server"], apiBeacon)
	return request(address, data)
}

// Return the file bytes for the requested payload.
func (contact API) GetPayloadBytes(profile map[string]interface{}, payload string) []byte {
    var payloadBytes []byte
    output.VerbosePrint(fmt.Sprintf("[*] Fetching new payload bytes: %s", payload))
    server := profile["server"]
    platform := profile["platform"]
    if server != nil && platform != nil {
		address := fmt.Sprintf("%s/file/download", server.(string))
		req, _ := http.NewRequest("POST", address, nil)
		req.Header.Set("file", payload)
		req.Header.Set("platform", platform.(string))
		client := &http.Client{}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == ok {
			buf, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				payloadBytes = buf
			}
		}
    }

	return payloadBytes
}

//C2RequirementsMet determines if sandcat can use the selected comm channel
func (contact API) C2RequirementsMet(profile map[string]interface{}, criteria map[string]string) bool {
	output.VerbosePrint(fmt.Sprintf("Beacon API=%s", apiBeacon))
	return true
}

//SendExecutionResults will send the execution results to the server.
func (contact API) SendExecutionResults(profile map[string]interface{}, result map[string]interface{}) {
	address := fmt.Sprintf("%s%s", profile["server"], apiBeacon)
	profileCopy := make(map[string]interface{})
	for k,v := range profile {
		profileCopy[k] = v
	}
	results := [1]map[string]interface{}{result}
	profileCopy["results"] = results
	data, _ := json.Marshal(profileCopy)
	request(address, data)
}

func request(address string, data []byte) []byte {
	req, _ := http.NewRequest("POST", address, bytes.NewBuffer(util.Encode(data)))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	body, _ := ioutil.ReadAll(resp.Body)
	return util.Decode(string(body))
}