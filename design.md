This file has some notes on what the program should look like and upcoming
planned work at a high level.

## Functionality TODOs

- [ ] Request var capture functionality on send
- [ ] History resource commands
- [ ] History functionality
- [ ] Cookie persistence
- [ ] Cookie commands
- [ ] Var resource commands (basic)


## Full Command Set

This is a listing of planned commands and whether they are currently
impelmented. This lists whether the command is implemented.

### Basic Commands

- [x] `suyac` - You can call `--version` to get current version.

### Project Manipulation

- [x] `suyac init` - Quickly create project with history & session files.
- [x] `suyac req` - List the request templates in the project.
- [x] `suyac req new` - Create a new request template in the poject.
- [x] `suyac req delete` - Delete an existing request template.
- [x] `suyac req show` - Show details on a specific request template.
- [ ] `suyac req edit` - Modify a request template. Changing name must be careful to not break history.
- [x] `suyac req caps` - List variable captures that are a part of the request template.
- [x] `suyac req caps delete` - Delete an existing var capture.
- [ ] `suyac req caps edit` - Update a var capture.
- [x] `suyac req caps new` - Create a new var capture in the template.
- [x] `suyac proj` - Show details on the project.
- [ ] `suyac proj edit` - Modify the project. Setting var prefix must be possible.
- [x] `suyac proj new` - Create a new project file with settable options.
- [x] `suyac hist` - Show basic info on the history such as file location and number of entries. Filterable by req.
- [x] `suyac hist show` - Show a single history entry.
- [x] `suyac hist clear` - Clear history. Filterable.
- [x] `suyac hist off` - Disable saving to history while still tracking any existing history file.
- [x] `suyac hist on` - Enable saving to history.
- [x] `suyac var [NAME] [VALUE]` - By self, list basic info about the var store, with breakdown by env.
- [x] `suyac var -d` - Delete a variable in the current environment. Flags can specify deletion everywhere.
- [x] `suyac env` - Set or get the current var environment. If given one that does not exist, it is created.
- [ ] `suyac flow` - Show details on flows and list them out.
- [ ] `suyac flow new` - Create a new flow in the project.
- [ ] `suyac flow show` - Show a particular flow's details.
- [ ] `suyac flow delete` - Delete an existing flow.
- [ ] `suyac flow edit` - Modify a flow.
- [ ] `suyac cookies` - Show the cookies that are currently stored.
- [ ] `suyac cookies clear` - Clear all current cookies.

### Projectless State Manipulation

- [x] `suyac state` - View/modify project-less state files.
- [ ] `suyac state cookies` - Show cookies in the session.
- [ ] `suyac state cookies clear` - Clear cookies in the session.
- [ ] `suyac state var` - List vars in a state file.
- [ ] `suyac state var set` - Set var in state file.
- [ ] `suyac state var get` - Show var in a state file.
- [ ] `suyac state var delete` - Delete a var in the state file.

### Request Sending

- [x] `suyac send` - Send a `req`.
- [ ] `suyac exec` - Send a flow (a sequence of request templates).
- [x] `suyac request` - (NAME SUBJECT TO CHANGE) Send an ad-hoc one-off request.
- [x] `suyac get` - Shorthand for `suyac request -X GET`.
- [x] `suyac post` - Shorthand for `suyac request -X POST`.
- [x] `suyac patch` - Shorthand for `suyac request -X PATCH`.
- [x] `suyac put` - Shorthand for `suyac request -X PUT`.
- [x] `suyac delete` - Shorthand for `suyac request -X DELETE`.
- [x] `suyac options` - Shorthand for `suyac request -X OPTIONS`.
- [x] `suyac head` - Shorthand for `suyac request -X HEAD`.
- [x] `suyac trace` - Shorthand for `suyac request -X TRACE`.




## project persisted files


A `project` includes named routes with consistent flow and is built much like in
Postman or Insomnia. That is the only place capture vars are stored (but they
can be set on a one-time basis with flags).
A `session` includes active cookies and current vars. It is something of an
invocation of either a project or a CLI one-time. If using a project, a default
session location is specified.
A `history` is stored separately. It includes request/response history.

When a PROJECT is referenced, `suyac send ENTRYNAME` can be used to fire a
request off.