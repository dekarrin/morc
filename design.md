This file has some notes on what the program should look like and upcoming
planned work at a high level.




## project vs sessions:


A `project` includes named routes with consistent flow and is built much like in
Postman or Insomnia. That is the only place capture vars are stored (but they
can be set on a one-time basis with flags).
A `session` includes active cookies and current vars. It is something of an
invocation of either a project or a CLI one-time. If using a project, a default
session location is specified. Sessions should also include request/response
history.
* suyac env read path/to/filename
- read state data and what is in it without actually doing anyfin


PROJECT:
contains:
- request entries
  - captures
  - name
  - body
  - url
  - headers
  - auth-flow (pre-call flow done to do any auth required)
- flows
  - reqentrys
- environments
  - variables
- current env
- history
- sesh-file

When a PROJECT is referenced, `suyac send ENTRYNAME` can be used to fire a
request off.