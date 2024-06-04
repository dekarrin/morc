// Package cliflags contains CLI flags. They may be referenced by multiple
// commands.
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

	// Get requests an attribute of a resource. It takes the attribute as an
	// argument.
	Get string

	// GetHeader requests the value(s) of the header of a resource. It takes the
	// name of the header as an argument.
	GetHeader string

	// Env specifies environment to apply to.
	Env string

	// Vars is variables, in NAME=VALUE format. Can be specified more than once.
	Vars []string

	// RemoveHeaders is a list of headers to be removed.
	RemoveHeaders []string

	// BRemoveBody is a switch flag that when set, indicates that the body of
	// the resource is to be removed.
	BRemoveBody bool

	// BForce is a switch flag that indicates that the requested operation
	// should proceed even if it is destructive or leads to a non-pristine
	// state.
	BForce bool

	// BDefault is a switch flag that, when set, indicates that the requested
	// operation should be applied to the default environment.
	BDefault bool

	// BNew is a switch flag that, when set, indicates that a new resource is
	// being created. Resource creations should generally use [New] instead,
	// but this flag is used when the resource is not required to have a name.
	BNew bool

	// BDeleteAll is a switch flag that, when set, indicates that all applicable
	// resources of the given type should be deleted.
	BDeleteAll bool

	// BCurrent is a switch flag that, when set, indicates that the requested
	// operation should be applied to the current environment explicitly.
	BCurrent bool

	// BAll is a switch flag that, when set, indicates that the requested
	// operation should be done with all instances of the applicable resource.
	BAll bool

	// BInsecure is a switch flag that, when set, disables TLS certificate
	// verification, allowing requests to go through even if the server's
	// certificate is invalid.
	BInsecure bool
)
