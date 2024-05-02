This file has some notes on what the program should look like and upcoming
planned work at a high level.



## Main Things


## Output Control
Need to be able to specify exactly what is shown.

Options:
* human - current state
* line-based - takes same extra output/suppress options but puts special separator between each (probably line-based).
* json - everything is in JSON


SHORT-STATUS - contains 'small' things in response like status, code, etc as specified by cli flags.
Whatever else there is, short-status comes first, everything else next, then response body at
each item that is not short status starts with a line that has a one-word desc of what is next and unambiguously starts
with a thing that will not be in a line of the request, user-settable, and ends with a similar thing


* json - everything is contained in JSON.



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