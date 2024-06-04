// Package cliflags contains flags common to an entire hierarchy of commands.
package cliflags

var (
	// ProjectFile is the flag that specifies the project file to use while
	// calling `morc req` or subcommands.
	ProjectFile string

	// New takes as argument the name of the resource being created.
	New string

	// Delete requests the deletion of a resource. It takes as argument the name
	// of the resource being deleted.
	Delete string

	// Env specifies environment to apply to.
	Env string
)
