package errors

import "fmt"

type PlatformNameError struct {
	Name string
}

func (err PlatformNameError) Error() string {
	return fmt.Sprintf("no such platform %s", err.Name)
}

type IncorrectPlatformError struct {
	Name   string
	Action string
}

func (err IncorrectPlatformError) Error() string {
	if len(err.Action) == 0 {
		err.Action = "UNKOWN"
	}
	if len(err.Name) == 0 {
		err.Name = "UNKOWN"
	}

	return fmt.Sprintf("incorrect platform; platform %s is unable to perform action %s", err.Name, err.Action)
}
