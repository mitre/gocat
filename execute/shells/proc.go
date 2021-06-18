package shells

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"github.com/google/shlex"

	"github.com/mitre/gocat/execute"
	"github.com/mitre/gocat/output"
)

type Proc struct {
	name string
	currDir string
}

func init() {
	cwd, _ := os.Getwd()
    executor := &Proc{
		name: "proc",
		currDir: cwd,
	}
	execute.Executors[executor.name] = executor
}

func (p *Proc) Run(command string, timeout int, info execute.InstructionInfo) ([]byte, string, string, time.Time) {
	exePath, exeArgs, err := p.getExeAndArgs(command)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("[!] Error parsing command line: %s", err.Error()))
		return nil, "", "", time.Now()
	}
	output.VerbosePrint(fmt.Sprintf("[*] Starting process %s with args %v", exePath, exeArgs))
	return runShellExecutor(*exec.Command(exePath, append(exeArgs)...), timeout)
}

func (p *Proc) String() string {
	return p.name
}

func (p *Proc) CheckIfAvailable() bool {
	return true
}

func (p *Proc) DownloadPayloadToMemory(payloadName string) bool {
	return false
}

func (p *Proc) getExeAndArgs(commandLine string) (string, []string, error) {
	if runtime.GOOS == "windows" {
		commandLine = strings.ReplaceAll(commandLine, "\\", "\\\\")
	}
	split, err := shlex.Split(commandLine)
	if err != nil {
		return "", nil, err
	}
	return split[0], split[1:], nil
}