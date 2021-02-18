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

func getHostIPAddrs() ([]string, error) {
    ifaces, err := net.Interfaces()
    if err != nil {
        return nil, err
    }
    var ipAddrs []string
    for _, i := range ifaces {
        addrs, err := i.Addrs()
        if err != nil {
            return nil, err
        }
        for _, addr := range addrs {
            var ip net.IP
            switch v := addr.(type) {
            case *net.IPNet:
                    ip = v.IP
            case *net.IPAddr:
                    ip = v.IP
            }
            ipv4 := ip.To4()
            if ipv4 != nil && !ip.IsLoopback() {
                ipAddrs = append(ipAddrs, ipv4.String())
            }
        }
    }
    return ipAddrs, nil
}
