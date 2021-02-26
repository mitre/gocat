package agent

import (
	"os"
	"os/user"
	"os/exec"
	"net"
)

// Checks for a file
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

// Creates payload from []bytes
func writePayloadBytes(location string, payload []byte) error {
	dst, err := os.Create(location)
	if err != nil {
		return err
	} else {
		defer dst.Close()
		if _, err = dst.Write(payload); err != nil {
			return err
		} else if err = os.Chmod(location, 0700); err != nil {
			return err
		} else {
			return nil
		}
	}
}

func getUsername() (string, error) {
	if userInfo, err := user.Current(); err != nil {
		if usernameBytes, err := exec.Command("whoami").CombinedOutput(); err == nil {
			return string(usernameBytes), nil
		} else {
			return "", err
		}
	} else {
		return userInfo.Username, nil
	}
}

// Return list of local IPv4 addresses for this machine (exclude loopback and unspecified addresses)
func getLocalIPv4Addresses() ([]string, error) {
	var localIpList []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			var ipAddr net.IP
			switch v:= addr.(type) {
			case *net.IPNet:
				ipAddr = v.IP
			case *net.IPAddr:
				ipAddr = v.IP
			}
			if ipAddr != nil && !ipAddr.IsLoopback() && !ipAddr.IsUnspecified() {
			    ipv4Addr := ipAddr.To4()
			    if ipv4Addr != nil {
				    localIpList = append(localIpList, ipv4Addr.String())
			    }
			}
		}
	}
	return localIpList, nil
}
