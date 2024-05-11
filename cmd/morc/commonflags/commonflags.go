// Package commonflags contains flags common to an entire hierarchy of commands.
package commonflags

var (
	// ReqProjectFile is the flag that specifies the project file to use while
	// calling `morc req` or subcommands.
	ReqProjectFile string
)
