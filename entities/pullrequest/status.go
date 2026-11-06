package pullrequest

import (
	"fmt"
	"strings"
)

// Status is the state of a pull request.
type Status int

const (
	_ Status = iota
	OpenStatus
	ClosedStatus
	MergedStatus
)

func (s Status) String() string {
	switch s {
	case OpenStatus:
		return "open"
	case ClosedStatus:
		return "closed"
	case MergedStatus:
		return "merged"
	default:
		return "unknown status"
	}
}

func StatusFromString(str string) (Status, error) {
	switch strings.ToLower(strings.TrimSpace(str)) {
	case "open":
		return OpenStatus, nil
	case "closed":
		return ClosedStatus, nil
	case "merged":
		return MergedStatus, nil
	default:
		return 0, fmt.Errorf("unknown status")
	}
}

func (s Status) Validate() error {
	if s != OpenStatus && s != ClosedStatus && s != MergedStatus {
		return fmt.Errorf("invalid")
	}
	return nil
}
