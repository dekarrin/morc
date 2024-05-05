v0.0.1 - May 5, 2024
--------------------
* Initial release. This version contains a handful of commands useable as a
minimum persisted set of sessions that can be re-run.
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
