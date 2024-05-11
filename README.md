MORC
====

Command-line based REST client for testing. Built out of frustration with tools
such as Postman and Insomnia starting as free and useful programs and eventually
moving to for-profit models when all I used them for was a quick testing
environment. This tool was created to fulfill the need for testing and provide a
way to do so from CLI as a bonus. The name Morc comes from more-curl, or, a CLI
MORonic Client due to it not being very fancy.

Installation
------------

Get a distribution from the releases page of the GitHub repo for this project
and untar it. Place the `morc` command somewhere on your path.

Usage
-----

Here are basic descriptions of commands, call `morc help` or
`morc help COMMAND` for info on CLI flags, etc.

### One-off Requests

Morc can send one-off requests by using `morc request`:

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

### Project management

Commonly sent requests can be collected as templates in a *project*. Body data,
headers, URL, and method are saved at creation and later sent with `morc send`.

First, a project is created with `morc init`. This puts all project files by
default in a new `.morc` directory in the directory it is called from.

Then, call `morc reqs new` to create a new request, giving the name of request.

Finally, at a later time, call `morc send` to actually fire it.

As an example:

```shell
morc init   # create the project, if it doesn't yet exist

morc reqs new get-google --url http://google.com/ -X GET

morc send get-google  # actually fire it off
```

Templating within a body or url or header is supported. Use variables in form of
`${NAME}` and supply values during a call to `morc send` with `-V`.

Saving variables during a `morc send` automatically is supported via the
concept of Variable Captures.

### Variables

Request templates within Morc can have variables within them that are filled at
send time. Variables are given in the format `${NAME}`, with NAME replaced by
the actual name of the variable.

```
(example wip)
```

When a template with one or more variables is sent, the values are substituted
in by drawing from one or more sources. First, all `-V` flags are checked for a
match. If found, that value is used. If there are no flags setting the value,
the current variable environment is checked for a value. If none is set, the
default environment is checked. If there is still no value, the template cannot
be filled, and an error is emitted.

Variables can also be set, viewed, and modified using the `morc vars` and
`morc env` commands.

#### Captures

Variable values can be taken from the response of a request.

(wip, fill in later)

### Flows

Flows are sequences of requests that will be fired one after another. It can be
useful to use with variable captures to perform a full sequence of communication
with a server.

Use `morc flows new` to create a new one. Once created, `morc exec FLOW` will
actually send off each request. Any variable captures from request sends are
used to set the values of subsequent requests.
