package util

import (
	"encoding/base64"
	//"encoding/hex"
	"encoding/json"
	//"io"
	//"net/http"
	//"fmt"
	"os"
	"path/filepath"
	//"strings"
	"time"
	//"unicode"
)

// Encode base64 encodes bytes
func Encode(b []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(b))
}

// Decode base64 decodes a string
func Decode(s string) []byte {
	raw, _ := base64.StdEncoding.DecodeString(s)
	return raw
}

// Unpack converts bytes into JSON
func Unpack(b []byte) (out map[string]interface{}) {
	_ = json.Unmarshal(b, &out)
	return
}

// Exists checks for a file
func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

// Sleep sleeps for a desired interval
func Sleep(interval float64) {
	time.Sleep(time.Duration(interval) * time.Second)
}

//WritePayloadBytes creates payload from []bytes
func WritePayloadBytes(location string, payload []byte) {
	dst, _ := os.Create(location)
	defer dst.Close()
	_, _ = dst.Write(payload)
	os.Chmod(location, 0700)
}

//CheckPayloadsAvailable determines if any payloads are not on disk
func CheckPayloadsAvailable(payloads []string) []string {
	var missing []string
	for i := range payloads {
		if Exists(filepath.Join(payloads[i])) == false {
			missing = append(missing, payloads[i])
		}
	}
	return missing
}

//StopProcess kills a PID
func StopProcess(pid int) {
	proc, _ := os.FindProcess(pid)
	_ = proc.Kill()
}


func EvaluateWatchdog(lastcheckin time.Time, watchdog int) {
	if watchdog > 0 && float64(time.Now().Sub(lastcheckin).Seconds()) > float64(watchdog) {
		StopProcess(os.Getpid())
	}
}