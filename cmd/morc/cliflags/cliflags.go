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

	// Name is the name of the resource in question.
	Name string

	// VarName is identical to Name but is named differently for readability.
	VarName string

	// BodyData is the bytes of the body of a request. This is either the bytes
	// of the body directly or a filename prepended with an '@' character.
	BodyData string

	// Headers is a list of headers to be added to the request.
	Headers []string

	// Method is the HTTP method to use for the request.
	Method string

	// URL is the URL to send the request to.
	URL string

	// HistoryFile is the path to a history file. It may contain the special
	// string "::PROJ_DIR::"; if so, it will be interpreted as the current
	// directory of the project file at runtime.
	HistoryFile string

	// SessionFile is the path to a history file. It may contain the special
	// string "::PROJ_DIR::"; if so, it will be interpreted as the current
	// directory of the project file at runtime.
	SessionFile string

	// CookieLifetime is a duration string that specifies the maximum lifetime
	// of recorded Set-Cookie instructions.
	CookieLifetime string

	// RecordHistory is a toggle-string flag that indicates whether history
	// recording should be "ON" or "OFF".
	RecordHistory string

	// RecordCookies  is a toggle-string flag that indicates whether cookie
	// recording should be "ON" or "OFF".
	RecordCookies string

	// WriteStateFile is a flag used in one-off commands that gives the path to
	// a state file to write out cookies and variables to.
	WriteStateFile string

	// ReadStateFile is a flag used in one-off commands that gives the path to a
	// state file to read cookies and variables from.
	ReadStateFile string

	// CaptureVars is a flag used in one-off commands that specifies a variable
	// to capture from the response. It can be specified multiple times.
	CaptureVars []string

	// BNoDates is an output control switch flag that indicates that dates of
	// historical events should not be printed when they otherwise would.
	BNoDates bool

	// BInfo is a switch flag that indicates that the requested operation is
	// retrieval of a summary of the resource.
	BInfo bool

	// BEnable is a switch flag that indicates that the requested operation is
	// to enable a feature.
	BEnable bool

	// BDisable is a switch flag that indicates that the requested operation is
	// to disable a feature.
	BDisable bool

	// BClear is a switch flag that indicates that the requested operation is to
	// erase all instances of the applicable type of resource.
	BClear bool

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
