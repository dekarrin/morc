MORC
====

Command-line based REST client for testing. Built out of frustration with tools
such as Postman and Insomnia starting as free and useful programs and eventually
moving to for-profit models when all I used them for was a quick testing
environment. This tool was created to fulfill the need for testing and provide a
way to do so from CLI as a bonus. The name Morc comes from more-curl, or, a CLI
MORonically-simple Client due to it not being very fancy.

Installation
------------

Get a distribution from the releases page of the GitHub repo for this project
and untar it. Place the `morc` command somewhere on your path.

Quickstart
----------

Send a one-off request:

```shell
morc request www.example.com -X PUT -u www.example.com -d @datafile.json -H 'Content-Type: application/json'
```

Send a request using a project:

```shell
morc init   # create the project, if it doesn't yet exist
morc reqs new get-google --url http://google.com/ -X GET
morc send get-google  # actually fire it off
```

Usage
=====

MORC has two primary ways that it can be used: project-oriented, or standalone
request sending. With project-oriented use, MORC operates within the context of
a *project* to create named request templates that can be sent as many times as
desired by referencing them by name. It tends to require more initial setup, but
is suitable for saving testing flows as data. Standalone-oriented use avoids the
use of separate project but requires that the entire request be specified every
time it is sent; using this mode is more similar to raw curl usage, with some
optional support for saving basic state info in between requests.

## Project-Oriented Use

MORC is generally designed to operate on a MORC *project*. A project has
requests and flows of requests defined within it that can be sent multiple times
without having to fully specify them each time they are sent. This is similar to
what you'd see in the main view of a GUI-based REST client, such as Postman or
Insomnia.

Beyond creating and sending requests, a project tracks sent request history and
sets of variables that can be set from the response of a request and used in
later requests. This, combined with defining sequences of requests in *flows*,
allows entire testing sequences to be defined and then run on-demand, which can
be useful for automated testing scenarios.

### Initializing A MORC Project

A MORC project is created with `morc init`. This puts all project files by
default in a new `.morc` directory in the directory that `morc` is called from,
and sets up cookie storage and history tracking.

```shell
morc init
```

If you want, you can give a name to the new project; otherwise, MORC will fall
back to using a default one.

```shell
morc init 'Testing Suite'
```

Now you can see the details of the project by running `morc proj` from the same
directory:

```shell
morc proj
```

Output:

```
Project: Unnamed Project
0 requests, 0 flows
0 history items
0 variables across 1 environment
0 cookies in active session

Cookie record lifetime: 24h0m0s
Project file on record: .morc/project.json
Session file on record: ::PROJ_DIR::/session.json
History file on record: ::PROJ_DIR::/history.json
Cookie recording is ON
History tracking is ON

Using default var environment
```

If you want to change things about the project, you can do that with the edit
subcommand:

```shell
morc proj edit --name 'My Cool Project'
```

Or if you are looking for *very* fine-grained control over new project creation,
you can use the `morc proj new` command. See `morc help proj new` for
information on running it.

### Project Requests

So, you've got a project rolling! Congrats. Now you can take a look at all the
requests that are loaded into it:

```shell
morc reqs
```

If this is in a brand new project, there won't be anything there.

#### Request Creation

You can add a new request with the `new` subcommand:

```shell
morc reqs new create-user --url localhost:8080/users -X POST -d '{"name":"Vriska Serket"}' -H 'Content-Type: application/json'
```

The URL, method, body payload, and headers can be specified with flags.
Alternatively, if you want to load the body from a file, put '@' followed by the
file name as the argument for `-d` and it will load the body data from that
file and use that as the body of the newly-created request:

```shell
morc reqs new update-user --url localhost:8080/users -X PATCH -d '@vriska.json' -H 'Content-Type: application/json'
```

After adding several requests, `morc reqs` will have much more interesting
output:

```shell
morc reqs
```

Output:

```
POST   create-user
GET    get-token
GET    get-user
GET    list-users
DELETE remove-user
PATCH  update-user
```

Each request name is listed along with the HTTP method that the request is
configured to use.

#### Request Sending

Once a request is set up in a project and has at least a method and a URL
defined on it, it can be sent to the remote server and the response can be
viewed.

Use the `send` subcommand with the name of the request to be send.

```shell
morc send list-users
```

Output:

```
HTTP/1.1 200 OK
[
    {"name": "Vriska"},
    {"name": "Nepeta"},
    {"name": "Kanaya"},
    {"name": "Terezi"}
]
```

The remote server's response as well as any body in the payload will be shown.
There's a lot of options to view additional details, such as seeing the headers
in the response, outputting the request, and format selection all available as
CLI options; take a look at `morc help send` to see them all.

If there are any variables in the request body, URL, or headers, they are filled
with their current values before the request is sent. See the section on Using
Variables below for more information on using variables within requests.

#### Request Viewing

You can examine a request in detail with the `show` subcommand:

```shell
morc reqs show create-user
```

Output:

```
POST http://example.com

HEADERS:
Content-Type: application/json

BODY:
{"name": "Vriska Serket"}

VAR CAPTURES: (NONE)

AUTH FLOW: (NONE)
```

The request method and URL are shown first, along with any headers, body, and
variable captures. Auth flow is for an upcoming feature and is not currently
used.

To see only one of the items in a request, you can specify it as a CLI flag:

```shell
morc reqs show create-user --body
```

Ouput:

```
{"name": "Vriska Serket"}
```

#### Request Editing

If you need to update a request, use the `edit` subcommand:

```shell
morc reqs edit create-user -d '{"name": "Nepeta Leijon"}'
```

You can use `show` to confirm that the update was applied:

```shell
morc reqs show create-user --body
```

Output:

```
{"name": "Nepeta Leijon"}
```

#### Request Deletion

If you're totally done with a request and want to permanently remove it from the
project, use the `delete` subcommand:

```shell
morc reqs delete get-token
```

It will be cleared from the project, which you can confirm by listing the
requests:

```shell
morc reqs
```

```
POST   create-user
GET    get-user
GET    list-users
DELETE remove-user
PATCH  update-user
```

### Using Variables

MORC supports the use of *variables* in requests. These are values in requests
that are filled in only when actually sending the request, and they can be
changed in between sends. Perhaps, for instance, you'd like to be able to swap
whether a request is sent via a plaintext HTTP request or using TLS (HTTPS). You
could do that by declaring a variable called `${SCHEME}` in the URL of the
request:

```shell
morc reqs edit get-user --url '${SCHEME}://localhost:8080/users'

# make sure to put text with a variable in it in single quotes so your shell
# doesn't try to interpret it itself
```

Then, you just need to make sure that the value for the variable is available
when sending it.

The simplest way is to provide it with `-V` when sending the request:

```shell
morc send get-user -V SCHEME=https --request  # --request will print the request as it is sent
```

Output:

```
------------------- REQUEST -------------------
(https://localhost:8080/users)
GET /users HTTP/1.1
Host: localhost:8080
User-Agent: Go-http-client/1.1
Accept-Encoding: gzip


(no request body)
----------------- END REQUEST -----------------
HTTP/1.1 200 OK
(no response body)
```

The scheme isn't included in the HTTP request itself, but it is shown in the
line just above the request proper. It's set to HTTPS, because `${SCHEME}` was
substituted with the user-supplied variable.

All variables have the form `${SOME_NAME}` when used inside a request, with
SOME_NAME replaced by the actual name of the variable (or `var` for short). They
are supported in the URL, body data, and headers of a request.

When substituting a var in a request in preperation for sending, MORC will check
in a few different places for values for that var. First, it will use any value
directly given by the CLI invocation with `-V`; this will override any values
stored within the project. Next, it will check to see if there are any *stored*
variables in the project that match, in the current variable environment. If
there are none, and the current variable environment is not the default, it will
check in the *default* variable environment. If it still can't find any values,
MORC will refuse to send the request.

#### Stored Variables



#### Variable Environments

#### Variable Capturing

Request templates within Morc can have variables within them that are filled at
send time. Variables are given in the format `${NAME}`, with NAME replaced by
the actual name of the variable.

When a template with one or more variables is sent, the values are substituted
in by drawing from one or more sources. First, all `-V` flags are checked for a
match. If found, that value is used. If there are no flags setting the value,
the current variable environment is checked for a value. If none is set, the
default environment is checked. If there is still no value, the template cannot
be filled, and an error is emitted.

Variables can also be set, viewed, and modified using the `morc vars` and
`morc env` commands.

WIP:
* Use of -V in send
* `morc vars` (etc)
* Description of vars envs
* `morc env`.

Saving variables during a `morc send` automatically is supported via the
concept of Variable Captures.

Variable values can be taken from the response of a request.

WIP:

* `morc reqs caps new`
* `morc reqs caps`
* `morc reqs caps edit`
* `morc reqs caps delete`

#### Creating Sequences Of Requests With Flows

Flows are sequences of requests that will be fired one after another. It can be
useful to use with variable captures to perform a full sequence of communication
with a server.

Use `morc flows new` to create a new one. Once created, `morc exec FLOW` will
actually send off each request. Any variable captures from request sends are
used to set the values of subsequent requests.

* `morc flows ...`
* `morc exec`

#### Request History

* `morc hist`

#### Cookie Store

* `morc cookies`

### Standalone Use

MORC can send one-off requests by using `morc request`:

```shell
morc request -X GET http://localhost/cool
```

Data and headers are specified with curl-like syntax:

```shell
morc request -X POST https://localhost/cool -H 'Content-Type: application/json' -d '@./datafile'
```

For convenience, top-level subcommands for each of the common eight HTTP methods
are defined. Calling one is exactly the same as calling
`morc request -X METHOD` and support all args except for '-X'.

```shell
morc get http://localhost:8080/cool  # same as morc request -X GET http://localhost:8080/cool
```
