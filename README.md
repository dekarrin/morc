suyac
=====

Command-line based REST client for testing. Built out of frustration with tools
such as Postman and Insomnia starting as free and useful programs and eventually
moving to for-profit models when all I used them for was a quick testing
environment. This tool was created to fulfill the need for testing and provide a
way to do so from CLI as a bonus. The name Suyac comes from suya-suya and curl.

Installation
------------

Get a distribution from the releases page of the GitHub repo for this project
and untar it. Place the `suyac` command somewhere on your path.

Usage
-----

Here are basic descriptions of commands, call `suyac help` or
`suyac help COMMAND` for info on CLI flags, etc.

### One-off Requests

Suyac can send one-off requests by using `suyac request`:

```shell
suyac request -X GET http://localhost/cool
```

Data and headers are specified with curl-like syntax:

```shell
suyac request -X POST https://localhost/cool -H 'Content-Type: application/json' -d '@./datafile'
```

For convenience, top-level subcommands for each of the common eight HTTP methods
are defined. Calling one is exactly the same as calling
`suyac request -X METHOD` and support all args except for '-X'.

```shell
suyac get http://localhost:8080/cool  # same as suyac request -X GET http://localhost:8080/cool
```

### Project management

Commonly sent requests can be collected as templates in a *project*. Body data,
headers, URL, and method are saved at creation and later sent with `suyac send`.

First, a project is created with `suyac init`. This puts all project files by
default in a new `.suyac` directory in the directory it is called from.

Then, call `suyac reqs new` to create a new request, giving the name of request.

Finally, at a later time, call `suyac send` to actually fire it.

As an example:

```shell
suyac init   # create the project, if it doesn't yet exist

suyac reqs new get-google --url http://google.com/ -X GET

suyac send get-google  # actually fire it off
```

Templating within a body or url or header is supported. Use variables in form of
`${NAME}` and supply values during a call to `suyac send` with `-V`.

Saving variables during a `suyac send` automatically is not yet supported, but
will be in a future release.
