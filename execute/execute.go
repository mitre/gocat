package execute

import (
	"encoding/base64"
	"path/filepath"
	"fmt"
	"os"
	"strings"
)

const (
	SUCCESS_STATUS 	= "0"
	ERROR_STATUS 	= "1"
	TIMEOUT_STATUS 	= "124"
	SUCCESS_PID 	= "0"
	ERROR_PID 	= "1"
)

type Executor interface {
	// Run takes a command string, timeout int, and instruction info.
	// Returns Raw Output, A String status code, and a String PID
	Run(command string, timeout int, info InstructionInfo) ([]byte, string, string)
	String() string
	CheckIfAvailable() bool
}

type InstructionInfo struct {
	Profile map[string]interface{}
	Instruction map[string]interface{}
}

func AvailableExecutors() (values []string) {
	for _, e := range Executors {
		values = append(values, e.String())
	}
	return
}

var Executors = map[string]Executor{}

//RunCommand runs the actual command
func RunCommand(info InstructionInfo, payloads []string) ([]byte, string, string) {
	encodedCommand := info.Instruction["command"].(string)
	executor := info.Instruction["executor"].(string)
	timeout := int(info.Instruction["timeout"].(float64))
	var status string
	var result []byte
	var pid string
	decoded, err := base64.StdEncoding.DecodeString(encodedCommand)
	if err != nil {
		result = []byte(fmt.Sprintf("Error when decoding command: %s", err.Error()))
		status = ERROR_STATUS
		pid = ERROR_STATUS
	} else {
		command := string(decoded)
		missingPaths := checkPayloadsAvailable(payloads)
		if len(missingPaths) == 0 {
			result, status, pid = Executors[executor].Run(command, timeout, info)
		} else {
			result = []byte(fmt.Sprintf("Payload(s) not available: %s", strings.Join(missingPaths, ", ")))
			status = ERROR_STATUS
			pid = ERROR_STATUS
		}
	}
	return result, status, pid
}

//checkPayloadsAvailable determines if any payloads are not on disk
func checkPayloadsAvailable(payloads []string) []string {
	var missing []string
	for i := range payloads {
		if fileExists(filepath.Join(payloads[i])) == false {
			missing = append(missing, payloads[i])
		}
	}
	return missing
}

// checks for a file
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}
