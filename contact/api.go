package contact

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/mitre/gocat/output"
)

var (
	apiBeacon = "/beacon"
)

//API communicates through HTTP
type API struct {
	name string
}

func init() {
	CommunicationChannels["HTTP"] = API{ name: "HTTP" }
}

//GetInstructions sends a beacon and returns response.
func (a API) GetBeaconBytes(profile map[string]interface{}) []byte {
	data, err := json.Marshal(profile)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Cannot request beacon. Error with profile marshal: %s", err.Error()))
		return nil
	} else {
		address := fmt.Sprintf("%s%s", profile["server"], apiBeacon)
		return request(address, data)
	}
}

// Return the file bytes for the requested payload.
func (a API) GetPayloadBytes(profile map[string]interface{}, payload string) []byte {
    var payloadBytes []byte
    server := profile["server"]
    platform := profile["platform"]
    if server != nil && platform != nil {
		address := fmt.Sprintf("%s/file/download", server.(string))
		req, err := http.NewRequest("POST", address, nil)
		if err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] Failed to create HTTP request: %s", err.Error()))
		} else {
			req.Header.Set("file", payload)
			req.Header.Set("platform", platform.(string))
			client := &http.Client{}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == ok {
				buf, err := ioutil.ReadAll(resp.Body)
				if err == nil {
					payloadBytes = buf
				} else {
					output.VerbosePrint(fmt.Sprintf("[-] Error reading HTTP response: %s", err.Error()))
				}
			}
		}
    }

	return payloadBytes
}

//C2RequirementsMet determines if sandcat can use the selected comm channel
func (a API) C2RequirementsMet(profile map[string]interface{}, criteria map[string]string) bool {
	output.VerbosePrint(fmt.Sprintf("Beacon API=%s", apiBeacon))
	return true
}

//SendExecutionResults will send the execution results to the server.
func (a API) SendExecutionResults(profile map[string]interface{}, result map[string]interface{}) {
	address := fmt.Sprintf("%s%s", profile["server"], apiBeacon)
	profileCopy := make(map[string]interface{})
	for k,v := range profile {
		profileCopy[k] = v
	}
	results := [1]map[string]interface{}{result}
	profileCopy["results"] = results
	data, err := json.Marshal(profileCopy)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Cannot send results. Error with profile marshal: %s", err.Error()))
	} else {
		request(address, data)
	}
}

func (a API) GetName() string {
	return a.name
}

func request(address string, data []byte) []byte {
	encodedData := []byte(base64.StdEncoding.EncodeToString(data))
	req, err := http.NewRequest("POST", address, bytes.NewBuffer(encodedData))
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Failed to create HTTP request: %s", err.Error()))
		return nil
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Failed to perform HTTP request: %s", err.Error()))
		return nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Failed to read HTTP response: %s", err.Error()))
		return nil
	}
	decodedBody, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Failed to decode HTTP response: %s", err.Error()))
		return nil
	}
	return decodedBody
}