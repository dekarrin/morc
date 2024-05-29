package morc

import "fmt"

func NewFlowNotFoundError(name string) error {
	if name == "" {
		name = `""`
	}
	return fmt.Errorf("no flow named %s exists in project", name)
}

func NewFlowExistsError(name string) error {
	if name == "" {
		name = `""`
	}
	return fmt.Errorf("flow named %s already exists in project", name)
}

func NewReqNotFoundError(name string) error {
	if name == "" {
		name = `""`
	}
	return fmt.Errorf("no request named %s exists in project", name)
}

func NewReqExistsError(name string) error {
	if name == "" {
		name = `""`
	}
	return fmt.Errorf("request named %s already exists in project", name)
}
