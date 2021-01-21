package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
	"math/rand"

	"github.com/mitre/gocat/agent"
	"github.com/mitre/gocat/output"

	_ "github.com/mitre/gocat/execute/donut" // necessary to initialize all submodules
	_ "github.com/mitre/gocat/execute/shells" // necessary to initialize all submodules
	_ "github.com/mitre/gocat/execute/shellcode" // necessary to initialize all submodules
)

// Initializes and returns sandcat agent.
func initializeCore(server string, group string, c2 map[string]string, p2pReceiversOn bool, initialDelay int, verbose bool, paw string, originLinkID int) (*agent.Agent, error) {
	output.SetVerbose(verbose)
	output.VerbosePrint("Starting sandcat in verbose mode.")
	return agent.AgentFactory(server, group, c2, p2pReceiversOn, initialDelay, paw, originLinkID)
}

//Core is the main function as wrapped by sandcat.go
func Core(server string, group string, delay int, c2 map[string]string, p2pReceiversOn bool, verbose bool, paw string, originLinkID int) {
	sandcatAgent, err := initializeCore(server, group, c2, p2pReceiversOn, delay, verbose, paw, originLinkID)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Error when initializing agent: %s", err.Error()))
		output.VerbosePrint("[-] Exiting.")
		return
	}
	sandcatAgent.Display()
	runAgent(sandcatAgent)
	sandcatAgent.Terminate()
}

// Establish contact with C2 and run instructions.
func runAgent (sandcatAgent *agent.Agent) {
	// Start main execution loop.
	sandcatAgent.UpdateCheckinTime(time.Now())
	lastDiscovery := time.Now()
	for (sandcatAgent.EvaluateWatchdog()) {
		// Send beacon and process response.
		beacon := sandcatAgent.Beacon()
		processBeaconResponse(sandcatAgent, beacon)

		// randomly check for dynamically discoverable peer agents on the network
		if findPeers(lastDiscovery, sandcatAgent) {
			lastDiscovery = time.Now()
		}
	}
}

func processBeaconResponse(sandcatAgent *agent.Agent, beacon map[string]interface{}) {
	// Process beacon response.
	if len(beacon) != 0 {
		sandcatAgent.SetPaw(beacon["paw"].(string))
		sandcatAgent.UpdateCheckinTime(time.Now())
		sandcatAgent.SetWatchdog(beacon["watchdog"].(int))
		sandcatAgent.MarkCurrCommsAsSuccessful()
	} else {
		// Failed beacon
		if err := sandcatAgent.HandleBeaconFailure(); err != nil {
			output.VerbosePrint(fmt.Sprintf("[!] Error handling failed beacon: %s", err.Error()))
		}
		sandcatAgent.Sleep(float64(15))
		return
	}

	// Check if we need to change contacts
	if beacon["new_contact"] != nil {
		changeAgentContact(sandcatAgent, beacon["new_contact"].(string))
	}

	// Handle instructions
	if beacon["instructions"] != nil && len(beacon["instructions"].([]interface{})) > 0 {
		handleInstructions(sandcatAgent, beacon["instructions"])
	} else {
		sandcatAgent.Sleep(float64(beacon["sleep"].(int)))
	}
}

func changeAgentContact(sandcatAgent *agent.Agent, newChannel string) {
	output.VerbosePrint(fmt.Sprintf("Received request to switch from C2 channel %s to %s", sandcatAgent.GetCurrentContactName(), newChannel))
	if err := sandcatAgent.SwitchC2Contact(newChannel, ""); err != nil {
		output.VerbosePrint(fmt.Sprintf("[!] Error switching communication channels: %s", err.Error()))
	}
}

func handleInstructions(sandcatAgent *agent.Agent, instructionsList interface{}) {
	// Run commands and send results.
	instructions := reflect.ValueOf(instructionsList)
	for i := 0; i < instructions.Len(); i++ {
		marshaledInstruction := instructions.Index(i).Elem().String()
		var instruction map[string]interface{}
		if err := json.Unmarshal([]byte(marshaledInstruction), &instruction); err != nil {
			output.VerbosePrint(fmt.Sprintf("[-] Error unpacking command: %v", err.Error()))
		} else {
			// If instruction is deadman, save it for later. Otherwise, run the instruction.
			if (instruction["deadman"].(bool)) {
				output.VerbosePrint(fmt.Sprintf("[*] Received deadman instruction %s", instruction["id"]))
				sandcatAgent.StoreDeadmanInstruction(instruction)
			} else {
				output.VerbosePrint(fmt.Sprintf("[*] Running instruction %s", instruction["id"]))
				droppedPayloads := sandcatAgent.DownloadPayloads(instruction["payloads"].([]interface{}))
				go sandcatAgent.RunInstruction(instruction, droppedPayloads, true)
				sandcatAgent.Sleep(instruction["sleep"].(float64))
			}
		}
	}
}

func findPeers(last time.Time, sandcatAgent *agent.Agent) bool {
    minDiscoveryInterval := 300
    diff := float64(time.Now().Sub(last).Seconds())
    if diff >= float64(rand.Intn(120) + minDiscoveryInterval) {
        sandcatAgent.DiscoverPeers()
        return true
    } else {
        return false
    }
}