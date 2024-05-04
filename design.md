This file has some notes on what the program should look like and upcoming
planned work at a high level.




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