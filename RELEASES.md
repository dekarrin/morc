v0.2.0 - May 16, 2024
---------------------
* Added `--url` flag to `morc cookies` to see only cookies for that URL.
* Required Go version for dev is now minimum 1.20 so we get unwrappable errors.
* Updated README.md to actually be a reasonable description of use that isn't
  just "go read the online help".
* Fixed numerous bugs and edge cases that made examples in README not work.
* Added new commands:
  * `morc proj edit` to edit a project.
  * `morc reqs edit` to edit a request template.
  * `morc reqs caps edit` to edit a var capture.

v0.1.0 - May 10, 2024
----------------------
* The name has been updated from `suyac` to `morc`, MORe than just Curl, the
MORonically simple Client.
* Added variable setting, persistence, and capturing, along with environments
with their own values for the same variables to quickly swap between sets of
them. Environments default to the default environment's value when a variable
isn't found.
* Added history saving and playback.
* Added cookie persistence.
* Added experimental support for **flows**, sequences of commands.
* The root command was updated from `suyac` to `morc`.
* The `req` subcommand was renamed to `reqs`.
* Added new commands:
  * `morc vars` and `morc env` for variable management.
  * `morc hist` for history management.
  * `morc cookies` for cookie management.
  * `morc reqs caps` for viewing template variable captures.
  * `morc reqs caps new` to create a new variable capture in a template.
  * `morc reqs caps delete` to remove a variable capture.
  * `morc flows` for viewing existing flows.
  * `morc flows new` to create a new flow.
  * `morc flows show` to see a particular flow.
  * `morc flows delete` to delete a flow.
  * `morc flows edit` to update a flow.
  * `morc exec` to execute a flow.


v0.0.1 - May 5, 2024
--------------------
* Initial release. This version contains a handful of commands usable as a
minimum persisted set of requests that can be re-run.
* Added new commands:
  * `suyac request` to send an arbitrary custom request that has a curl-like
  interface for specifying headers, data, etc. Basic variable substitution and
  capturing are supported, though capturing only outputs to stdout and does not
  actually store the values at this time. It can use a state file separate from
  the project to store cookies for future calls.
  * `suyac get`, `suyac post`, `suyac put`, `suyac patch`, `suyac delete`,
  `suyac options`, `suyac head`, `suyac trace` were all added as shorthand for
  `suyac request -X (THE METHOD)` and have the same flag semantics.
  * `suyac state` to view state files created by `suyac request` and the
  shorthand method versions.
  * `suyac init` to create a new set of history, session, and project files in
  `.suyac` in the current directory.
  * `suyac proj` to view the current state of the project.
  * `suyac proj new` to create a new project file without necessarily creating
  a history or session file.
  * `suyac req` to list the request templates in the current project.
  * `suyac req new` to create a new request template.
  * `suyac req delete` to delete an existing request template.
  * `suyac req show` to show details on an existing request template.
  * `suyac send` to actually fire off a request defined by a request template.
