v0.4.2 - August 30, 2024
------------------------
* Var capture specs may now specify an end offset of 0 to indicate an offset of
the end of the response, however long it is.
* Var capture specs may now specify a negative end offset to indicate an end
offset that many bytes away from the end of the response, however long that is.
* Var capture specs may omit the START or END parameter to indicate an offset of
0.
* Var capture specs now require a leading dot when giving a JSON path.
* Var capture specs may specify capturing the entire response with the special
keyword "raw"; this is equivalent to an offset that specifies a START and END of
zero.


v0.4.1 - August 29, 2024
------------------------
* Fixed bug where new captures could not be created due to caps command reading
the variable name from the incorrect flag.
* Fixed bug where creation of new variable capture would silently overwrite
existing ones due to case.
* `caps` and `env` output now end with newlines, and list output is
alphabetized.
* Request template names, flow names, and capture variable names will now have
their case normalized on load.
* Added automated functional tests to cover additional functions:
  * `morc caps` is now covered.
  * `morc env` is now covered.
* Added Go 1.23 to list of versions to run automated testing against.


v0.4.0 - August 10, 2024
------------------------
* The prefix used by variables is now an updatable project config setting.
* Added new flag `--var-prefix`/`-p` to allow overriding the
variable prefix to commands `morc send`, `morc exec`, and `morc oneoff`.
* Added alternative way of specifying project location using a file called
`.MORC_PROJECT` in the working directory of the command.
* All commands now give some kind of output.
* Added new `--quiet`/`-q` flag to all commands to suppress unnecessary output.
* Fixed `--project-file` in `morc vars` and docs being incorrectly set to
`--project_file`.
* Updated error reporting in `morc vars` to clearly note why an operation
failed and what flags could be used to perform the operation if desired.
* `morc vars` now supports `--default` during deletion.
* `morc vars` now supports `--all` during value retrieval.
* Added automated functional tests to cover additional functions:
  * `morc vars` is now covered.
  * `morc send` is now covered.
* Functional tests will now read and write the project files to memory buffers
rather than to a file unless explicitly testing file writing.
* Refactored all arg-parsing code to use a common pattern to aid in debugging.


v0.3.1 - June 5, 2024
---------------------
* All `--project_file` flags have been updated to `--project-file`.
* Fixed bug where `morc cookies` did not respect `--project-file`/`-F`.
* Fixed bug where `morc hist` did not respect `--project-file`/`-F`.
* Fixed bug where `morc caps` ignored the value of `--get`/`-G`.
* Fixed bug where `morc hist ENTRY` printed dates when `--no-dates` was set.
* Removed `morc oneoff` and friends' `--var-symbol` flag. It was being
inconsistently applied and could not be used in project-oriented use. It will be
restored in a future release where it will be added to all commands that
interpret templates.


v0.3.0 - June 4, 2024
---------------------
* Added new `--insecure`/`-k` flag to disable TLS certificate validation on
request-sending commands.
* Flow step indexes are now always 0-based in both printed output and
specification of steps in flags.
* Updated online help to be accurate, somewhat complete, and to wrap correctly
in console output.
* Started automatically creating a build for Apple-Silicon based Macs (darwin/arm64).
* The flag `--delete-all` was added to `morc env`. `--all` can no longer be used
during environment deletion, and exclusively lists all environments.
* Several flags have been added to many commands to allow editing via flags.
* Renamed `morc request` to `morc oneoff` to better distinguish from `reqs`.
* Altered command interface to make all subcommands go only one level deep:
  * `morc reqs delete REQ` was removed in favor of `morc reqs --delete REQ`.
  * `morc reqs edit REQ` was removed in favor of `morc reqs REQ [mutation-flags]`. 
  * `morc reqs new REQ` was removed in favor of `morc reqs --new REQ`.
  * `morc reqs show REQ` was removed in favor of `morc reqs REQ`.
  * `morc reqs caps REQ` was removed in favor of `morc caps REQ`.
  * `morc reqs caps delete REQ VAR` was removed in favor of `morc caps REQ --delete VAR`.
  * `morc reqs caps edit REQ VAR` was removed in favor of `morc caps REQ VAR [mutation-flags]`.
  * `morc reqs caps new REQ VAR SPEC` was removed in favor of `morc caps REQ --new VAR -s SPEC`.
  * `morc proj edit` was removed in favor of `morc proj [mutation-flags]`.
  * `morc proj new` was removed in favor of `morc proj --new`.
  * `morc flows delete FLOW` was removed in favor of `morc flows --delete FLOW`.
  * `morc flows edit FLOW` was removed in favor of `morc flows FLOW [mutation-flags]`. 
  * `morc flows new FLOW` was removed in favor of `morc flows --new FLOW`.
  * `morc flows show FLOW` was removed in favor of `morc flows FLOW`.
  * Deletion of project resources was standardized as flag `--delete`/`-D` that
  takes the name of the resource to be deleted as an argument.
  * Creation of project resources was standardized as flag `--new`/`-N` that
  takes the name of the resource to be created as an argument (except for
  `proj --new`, which does not take an argument).
  * Retrieval of specific attributes of project resources was standardized as
  flag `--get`/`-G` that takes the name of the attribute as an argument. The
  attributes that are possible to retrieve is different for each resources, and
  subcommand help as well as error output inform what the proper options are. 
* Some automated functional tests were added to cover at least key parts of project-oriented use:
  * `morc send` is covered under happy path only.
  * `morc flows` is covered.
  * `morc proj` is covered.
  * `morc reqs` is covered.


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
