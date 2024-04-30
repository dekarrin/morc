This file has some notes on what the program should look like.

Example executions:

* suyac get https://www.example.com - get the URL
* suyac post https://www.example.com -d '{text}' (or -d @file)
* suyac request custom https://www.example.com

-b - request state-file. Use cookies and other captured data items from here
-c - save state-file. Store cookies and other captured data to this file,
updating only the items that the server sends back. File is created if not
exists. Overrides any defaults.

-V - read data in the given path/offset and store it in the state file in a var.
Useless without -c. form is "name::2,4" for a byte offset (from 2-4 in example)
or "name:/path[3]" for json object path using jq-ish syntax but supporting only
indexes or exact object names.capture

-H - include a header with the given value. form is "Header-name: value".
Multiple headers with the same name may be specified. Use $ sign to reference a
stored variable in a file referred to by -b. Use double $$ to escape it.

-d - include a data payload. Use '@' followed by file name to read data payload
from file. Within a data payload, use $ sign to reference a stored variable in a
file referred to by -b, or use double $$ to escape it.

--var-symbol - change the var symbol from '$' to something else to avoid having
to do a lot of shell escaping. '@' is reserved and may not be supplied as arg.

--output-headers - include headers in output



* suyac state read path/to/filename
- read state data and what is in it without actually doing anyfin