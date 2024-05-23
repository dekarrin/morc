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
