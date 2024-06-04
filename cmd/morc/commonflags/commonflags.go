// Package commonflags contains flags common to an entire hierarchy of commands.
package commonflags

var (
	// ProjectFile is the flag that specifies the project file to use while
	// calling `morc req` or subcommands.
	ProjectFile string

	// New takes as argument the name of the thing being created.
	New string
)
