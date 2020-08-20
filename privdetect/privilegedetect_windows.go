// +build windows
package privdetect

import (
    "syscall"
    "unsafe"
    "fmt"
    "github.com/mitre/gocat/output"
)
type Token uintptr

const (
	// do not reorder
	TokenUser = 1 + iota
	TokenGroups
	TokenPrivileges
	TokenOwner
	TokenPrimaryGroup
	TokenDefaultDacl
	TokenSource
	TokenType
	TokenImpersonationLevel
	TokenStatistics
	TokenRestrictedSids
	TokenSessionId
	TokenGroupsAndPrivileges
	TokenSessionReference
	TokenSandBoxInert
	TokenAuditPolicy
	TokenOrigin
	TokenElevationType
	TokenLinkedToken
	TokenElevation
	TokenHasRestrictions
	TokenAccessInformation
	TokenVirtualizationAllowed
	TokenVirtualizationEnabled
	TokenIntegrityLevel
	TokenUIAccess
	TokenMandatoryPolicy
	TokenLogonSid
	MaxTokenInfoClass
	errnoERROR_IO_PENDING = 997
)

var (
	errERROR_IO_PENDING error = syscall.Errno(errnoERROR_IO_PENDING)
)

func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case errnoERROR_IO_PENDING:
		return errERROR_IO_PENDING
	}
	return e
}

func IsElevated(token syscall.Token) bool {
	var isElevated uint32
	var outLen uint32
	err := syscall.GetTokenInformation(token, TokenElevation, (*byte)(unsafe.Pointer(&isElevated)), uint32(unsafe.Sizeof(isElevated)), &outLen)
	if err != nil {
		output.VerbosePrint(fmt.Sprintf("Error getting token info: %s", err.Error()))
		return false
	}
	return outLen == uint32(unsafe.Sizeof(isElevated)) && isElevated != 0
}

func Privlevel() string{
    token, err := syscall.OpenCurrentProcessToken()
    if err != nil {
    	output.VerbosePrint(fmt.Sprintf("Error opening current process token: %s", err.Error()))
    	return "User"
    }
    if IsElevated(token) ==true {
    	return "Elevated"
    } else {
    	return "User"
    }
}