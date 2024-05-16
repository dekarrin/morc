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

# MAKE SURE to put text with a dollar-leading ${variable} in it in single quotes
# or your shell may mess with the variable
```

Then, you just need to make sure that the value for the variable is available
when sending it.

The simplest way is to provide it with `-V` when sending the request:

```shell
morc send get-user -V SCHEME=https --request  # --request will print the request as it is sent

# note that when providing a var like this, it does NOT start with a $, so you
# do not need to single-quote it.
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
are supported in the URL, body data, and headers of a request. Variables are
case-insensitive and their names can be made up of the letters, numbers,
underscores, and hyphens.

When substituting a var in a request in preperation for sending, MORC will check
in a few different places for values for that var. First, it will use any value
directly given by the CLI invocation with `-V`; this will override any values
stored within the project. Next, it will check to see if there are any *stored*
variables in the project that match, in the current variable environment. If
there are none, and the current variable environment is not the default, it will
check in the *default* variable environment. If it still can't find any values,
MORC will refuse to send the request.

#### Stored Variables

Variables do not always need to be provided at the time that you send request.
MORC maintains a variable store inside of project files that can hold the values
of variables indefinitately; they'll exist until the project is deleted, they
are deleted, or they are updated automatically via a variable capture.

The variable store is accessed and manipulated with the `vars` subcommand. By
itself, it will list all of the values that it would use for any sent request:

```shell
morc vars
```

With nothing defined, it will give output indicating that:

```
(none)
```

With variables set, it will them out:

```
${SCHEME} = "https"
${TEST_CAP} = "octyp"
${THETHING} = "doctyp"
```

Strictly speaking, there are actually *two* layers of variables possible within
the store; a currently-selected one, and a default one. These are organized as
separate *environments*, and they are covered in detail in the Variable
Environments section below. Unless it mentioned otherwise, this README will
assume you're working with the project set to use only the default environment,
as that is the situation when a project is first created.

#### Setting & Getting Variables

To set the value of a variable, give the name of the variable and the value as
arguments to `morc vars`:

```shell
morc vars USER_ID 24f6dc51-17ba-4aca-937c-52b40b9b715c
```

It will then be shown when listing all variables:

```shell
morc vars
```

Output:

```
${USER_ID} = "24f6dc51-17ba-4aca-937c-52b40b9b715c"
```

You can get only that variable's value by giving the name of the variable with
no value:

```shell
morc vars USER_ID
```

Output:

```
24f6dc51-17ba-4aca-937c-52b40b9b715c
```

Note that there is a difference between a variable being undefined and a
variable being defined and set to the empty string. Requests can use any
variable defined at send time, including any that are set to the empty string,
but they cannot be sent if they use variables that are undefined.

```shell
morc vars USER_ID ""

# ${USER_ID} can still be used in templates, even though its value is being set to ""
```

If you want to actually remove (undefine) a variable from the project var store,
use the `-d` option with the name of a variable:

```shell
morc vars USER_ID -d
```

Then the variable will be completely undefined:

```shell
morc vars USER_ID
```

Output:

```
"USER_ID" is not defined
```

The deleted (undefined) variable will be unusable in requests until it is
re-defined, but it also will no longer take up space in the project file.

#### Variable Capturing

MORC has the ability to automatically set the values of variables in the store
from data that it receives in response to a sent request. This makes MORC
capable of automating entire sequences of request activity; it could, for
instance, POST a new user with one request, then save the generated UUID of the
user in a var, then use that same UUID in subsequent requests that operate on
the user.

This is known as *Variable Capturing*. Each request in a project can have one or
more variables that it saves data to, known as "Variable Captures" or just
"caps" for short.

For the examples in this section, we will assume there is a Users API in a local
server that allows basic CRUD operations on its resources.

First, we will make a creation request:

```shell
morc reqs new create-user             \
  --url http://localhost:8080/users   \
  -X POST                             \
  -d '{"name": "Vriska Serket"}'      \
  -H 'Content-Type: application/json'
```

And a deletion request that uses a variable in the URL to indicate
which user to delete:

```shell
morc reqs new delete-user                          \
  --url 'http://localhost:8080/users/${USER_ID}'   \
  -X DELETE                                        \
  -H 'Content-Type: application/json'

# note that the URL is in single-quotes because it contains a variable
```

Now, with those requests, we certainly manually run the first:

```shell
morc send create-user
```

Output:

```
HTTP/1.1 201 Created
{
    "id": "92ed835e-252e-40e4-8aa3-423b496bf33d",
    "name": "Vriska Serket"
}
```

Then we could note the user ID of the new user, and supply it to the delete
request manually:

```shell
morc send delete-user -V USER_ID=92ed835e-252e-40e4-8aa3-423b496bf33d
```

Output:

```
HTTP/1.1 204 No Content
(no response body)
```

But we could also do the same thing automatically by adding a var capture to
the first request.

Captures on a request are accessed by using the `caps` subcommand of `reqs`. By
itself, it will list out all defined captures on the request, which will be
none so far:

```shell
morc reqs caps create-user
```

Output:

```
(none)
```

To add a new capture, use `caps new` followed by the name of the request, the
name of a variable to save the data to, and a *capture spec*. The capture spec
gives where in the response to retrieve the value from, and supports byte
offsets in format `:START,END` where START and END are byte offsets, or in
format of a JSON path specified by giving keys and array slices needed to
navigate from the top level of a JSON body in the response to the desired value,
such as `.top-level-key.next-level-key.some_array[3].item`.

In this example, we will add a new cap that gets its value from the 'id' field
of the JSON object in the response:

```shell
morc reqs caps new create-user USER_ID .id   # or just id with no period; the leading period is not required
```

Then, it will be created:

```shell
morc reqs caps create-user
```

Output:

```
USER_ID from .id
```

Now when the first request is sent, the value in .id is saved to USER_ID in the
variable store:

```shell
morc send create-user
```

Output:

```
HTTP/1.1 201 Created
{
    "id": "c2328061-da05-4241-9a42-012f2e39ff72",
    "name": "Vriska Serket"
}
```

You can check the var store to be sure:

```shell
morc vars
```

Output:

```
USER_ID = "c2328061-da05-4241-9a42-012f2e39ff72"
```

And now you can send the delete request without having to manually specify the
user:

```shell
morc send delete-user
```

Output:

```
HTTP/1.1 204 No Content
(no response body)
```

If you ever need to update a capture, you can do so with `caps delete`:

```shell
morc reqs caps delete create-user USER_ID
```

And if you want to update one without deleting it, you can use `caps edit`:

```shell
morc reqs caps edit create-user USER_ID --spec :3,8       # capture data from the 3rd to 8th byte instead of the JSON path
morc reqs caps edit create-user USER_ID --var USER_UUID   # save it to USER_UUID instead
```

#### Variable Environments

The variable store in MORC supports having multiple sets of vars that you can
easily switch between. Each of these sets is called an *environment*; they might
be set up to, say, change the requests to be applicable for testing different
deploy scenarios.

For instance, one might two sets of variables such as the following:

```
# set 1:

${BASE} = "http://staging.internal.example.com/api/v2"
${USER} = "myTestUser"

# set 2:

${BASE} = "https://example.com/api/v2"
${USER} = "internalTesting"
```

These two sets of vars could be assigned to two different environments in the
MORC var store to allow easy switching between testing against a staging server
and testing against a production server.

Whenever a variable in a MORC project is created, set, deleted, or used in a
request, it will be done in whatever environment MORC is set to use. When a
project is first started, this will be what is known as the
*default environment*.

You can check what environment MORC is in with the `env` subcommand:

```shell
morc env
```

Output:

```
<DEFAULT>
```

It gives the special string `<DEFAULT>` to indicate that MORC is currently set
to use the default environment.

To swap to a new environment, give the name of the environment to swap to.
You can swap to one that doesn't yet exist; using it will automatically create
it when necessary:

```shell
morc env STAGING
```

Names are case-insensitive, like variable names.

Once swapped to the new environment, you can start defining variables!

```shell
morc vars BASE http://staging.internal.example.com/api/v2
morc vars USER myTestUser
```

You can then swap to another environment to give them different values:

```shell
morc env PROD

morc vars BASE https://example.com/api/v2
morc vars USER internalTesting
```

Now the values can be easily swapped to be appropriate for each environment:

```shell
morc env STAGING

morc vars
```

Output:

```
${BASE} = "http://staging.internal.example.com/api/v2"
${USER} = "myTestUser"
```

```shell
morc env PROD

morc vars
```

Output:

```
${BASE} = "https://example.com/api/v2"
${USER} = "internalTest"
```

You can swap back to the default environment by giving `--default` as an option
to `env`:

```shell
morc env --default

morc vars
```

Output:

```
${BASE} = ""
${USER} = ""
```

The default environment will have a copy of every var that is set on any other
environment, but it will be set to the empty string unless explicitly set. More
info on the interactions between variables in the default environment and those
in others is available in the Defaulting section below.

There is shorthand for setting or viewing vars in a particular environment so
you do not need to swap *every* time just to perform a few operatoins. For
instance, when listing vars, you can pass in --env to list variable values
defined only in that environment, or --default to see it for the default
environment:

```shell
morc env STAGING
morc vars --env PROD
```

Output:
```
${BASE} = "http://staging.internal.example.com/api/v2"
${USER} = "myTestUser"
```

The same flag is available when setting or retrieving a particular variable's
value:

```shell
morc vars BASE http://staging.internal.example.com/api/v3 --env PROD
```

##### Defaulting

When MORC is in a non-default environment, the default environment still exists
and is used for getting "default" values of a variable. The default environment
will contain at least one copy of every single var that is defined in at least
one other environment, or put more simply, every time you set a variable in a
non-default environment, if it doesn't already exist in the default, it is added
there as well with a value of the empty string.

When in a non-default environment, it's possible to send a request that has
variables in it that are not defined in the current environment. In that case,
the default environment is consulted for its value on it and that is used if it
exists. This can make it easier when creating new environments in cases where
there are already several variables defined; instead of requiring you to
redefine every single one in the new environment, you only need to redefine the
ones you wish to update.

It's an invariant that if a variable is defined in at least one environment, it
will be defined in the default environment. As a result, *deleting* a variable
from the default environment is only possible if you also want to delete it from
every other environment that defines it, and this is required to be explicitly
stated via command line flags, or MORC will return an error.

```shell
# swap to env staging:
morc env staging

# create a new variable called PASSWORD in staging env; this will *also* create
# a var PASSWORD="" in the default environment
morc vars PASSWORD grimAuxiliatrix

# swap back to default environment
morc env --default

# attempt to delete PASSWORD from the default env:
morc vars PASSWORD -d
```

Output:

```
Error: current env is default and "PASSWORD" is defined in other envs: STAGING
Use --all to delete from all environments
```

If you're absolutely sure that you want to clear it from all environments, give
the --all flag as well:

```shell
morc vars PASSWORD -d --all
```

### Creating Sequences Of Requests With Flows

Flows are sequences of requests that will be fired one after another. It can be
useful to use with variable captures to perform a full sequence of communication
with a server.

Use `morc flows new` to create a new one. Once created, `morc exec FLOW` will
actually send off each request. Any variable captures from request sends are
used to set the values of subsequent requests.

### Request History

MORC projects maintain a history of requests and responses that were sent. If
the project was created with `morc init`, history will be enabled by default.

To view a list of all requests that the project has in its history, use `hist`
with no other options:

```shell
morc hist
```

Output:

```
0: 2024-05-12T08:13:09-05:00 - create-user - POST http://localhost:8080/users - 201 Created - 0s
1: 2024-05-13T08:50:57-05:00 - get-user - GET http://localhost:8080/users/c2328061-da05-4241-9a42-012f2e39ff72 - 200 OK - 0s
2: 2024-05-15T07:49:12-05:00 - list-users - GET http://localhost:8080/users - 200 OK - 1s
3: 2024-05-15T11:37:02-05:00 - update-user - PATCH http://localhost:8080/users/c2328061-da05-4241-9a42-012f2e39ff72 - 200 OK - 1s
4: 2024-05-15T11:39:32-05:00 - delete-user - DELETE http://localhost:8080/users/c2328061-da05-4241-9a42-012f2e39ff7 - 404 Not Found - 0s
4: 2024-05-15T11:39:38-05:00 - delete-user - DELETE http://localhost:8080/users/c2328061-da05-4241-9a42-012f2e39ff72 - 204 No Content - 1s
8: 2024-05-16T11:00:58-05:00 - create-user - POST http://localhost:8080/users - 200 OK - 0s
```

Each history entry begins with an entry index. A particular entry can be played
back by giving an entry index number, along with any other output formatting
options as would be accepted by `morc send` or `morc request`:

```shell
morc hist 0
```

Output:

```
Request template: create-user
Request sent:          2024-05-12T08:13:09-05:00
Response received:     2024-05-12T08:13:09-05:00
Total round-trip time: 0s
HTTP/1.1 201 Created
{
    "id": "c2328061-da05-4241-9a42-012f2e39ff72",
    "name": "Vriska Serket"
}
```

To turn off history recording, use the --off flag:

```shell
morc hist --off
```

To turn history recording on, use the --on flag:

```shell
morc hist --on
```

To check the current status of history and see a summary of the store, use the
--info flag:

```shell
morc hist --info
```

Output:

```
9 entries in .morc/history.json

History is ON
```

To clear out the current history store, either delete the file listed in
`--info` output from the filesystem, or use the `--clear` flag:

```shell
morc hist --clear
```

### Cookie Store

MORC projects save cookies received in responses from remote servers. Due to
internal limitations, it specifically tracks all cookies requested to be set by
a Set-Cookie header, not necessarily whether it would actually be sent.

MORC will save cookies for only as long as the currently-configured cookie
lifetime in the project is, regardless of when their expiry is set.

In a project created by `morc init`, cookie recording will be on and the cookie
lifetime will be defaulted to 24 hours.

To list all cookies that are currently in the session store, use the `cookies`
subcommand with no other arguments:

```shell
morc cookies
```

Output:

```
http://www.google.com/:
2024-05-15T11:37:03-05:00 1P_JAR=2024-05-15-16; Path=/; Domain=google.com; Expires=Fri, 14 Jun 2024 16:37:03 GMT; Secure
2024-05-15T11:37:03-05:00 AEC=AQTF6HwJeI71s5iqRHg95Jus2P9Vlc-e-eBtv9yCD0etgcjVzKxJPPo5upY; Path=/; Domain=google.com; Expires=Mon, 11 Nov 2024 16:37:03 GMT; HttpOnly; Secure; SameSite=Lax

https://www.google.com/:
2024-05-15T11:40:04-05:00 1P_JAR=2024-05-15-16; Path=/; Domain=google.com; Expires=Fri, 14 Jun 2024 16:40:04 GMT; Secure
```

If you want to see the exact cookies that would be sent in `Cookie:` headers on
a request to a particular URL, you can give it with `--url`:

```shell
morc cookies --url https://google.com/
```

Output:

```
1P_JAR=2024-05-15-16
AEC=AQTF6Hzwnh-yB8n0D7lG6i_2d9XqUyun2su9vCd_rTOQDFr1r3BQnpXN7Q
```

To turn session (cookie) recording off, use the --off flag:

```shell
morc cookies --off
```

To turn session (cookie) recording on, use the --on flag:

```shell
morc hist --on
```

To check the current status of cookies and see a summary of the session store,
use the --info flag:

```shell
morc cookies --info
```

Output:

```
3 cookies across 2 domains in .morc/session.json

Cookie recording is ON
```

To clear out the current session store, either delete the file listed in
`--info` output from the filesystem, or use the `--clear` flag:

```shell
morc cookies --clear
```

## Standalone Use

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
