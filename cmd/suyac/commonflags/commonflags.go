// Package commonflags contains flags common to an entire hierarchy of commands.
package commonflags

var (
	// ReqProjectFile is the flag that specifies the project file to use while
	// calling `suyac req` or subcommands.
	ReqProjectFile string
)
