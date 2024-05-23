package morc

import "fmt"

func NewFlowNotFoundError(name string) error {
	if name == "" {
		name = `""`
	}
	return fmt.Errorf("flow %s does not exist in project", name)
}

func NewFlowExistsError(name string) error {
	if name == "" {
		name = `""`
	}
	return fmt.Errorf("flow %s already exists in project", name)
}
