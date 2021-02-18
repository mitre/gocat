// +build windows

package shells

import (
	"github.com/mitre/gocat/execute"
	"os/exec"
)

type Cmd struct {
	shortName string
	path string
	execArgs []string
}

func init() {
	shell := &Cmd{
		shortName: "cmd",
		path: "cmd.exe",
		execArgs: []string{"/C"},
	}
	if shell.CheckIfAvailable() {
		execute.Executors[shell.shortName] = shell
	}
}

func (c *Cmd) Run(command string, timeout int, info execute.InstructionInfo) ([]byte, string, string) {
	return runShellExecutor(*exec.Command(c.path, append(c.execArgs, command)...), timeout)
}

func (c *Cmd) String() string {
	return c.shortName
}

func (c *Cmd) CheckIfAvailable() bool {
	return checkExecutorInPath(c.path)
}

func (c* Cmd) DownloadPayloadToMemory(payloadName string) bool {
	return false
}