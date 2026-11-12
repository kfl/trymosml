Webshell
========

Custom webservice for Try Moscow ML.

Production deployment uses Caddy for HTTPS and static files, and systemd for service management.
See [deployment instructions](../deploy/README.md) for installation, session
limits, testing, backups, and rollback. The Go server now serves plain HTTP only
and should bind to localhost behind Caddy.

Written in Go, the structure of the code is:

* `main.go` serves `/healthz` and starts WebSocket sessions at `/webshell`.
  
* `registry.go` keeps track of the open websocket sessions/

* `toplevel.go` deals with the communication to a (Moscow ML)
  toplevel.
  
* `client.go` sets up the communication between a toplevel and a
  websocket.
  
There should be little in the that is Moscow ML specific. Thus, the
`webshell` service could be used with other toplevels (REPLs).


How to build
------------

The `webshell` binary can be build be using the standard Go build
command:

~~~
$ go build
~~~


How to configure the webshell
-----------------------------

The `webshell` binary reads its configuration from a JSON file, which
can be specified with the `-conf` command-line argumend.  The file
should have the following form:

~~~.js
{
    "Addr": "127.0.0.1:8080",
    "Command": "mosml",
    "CommandArgs": ["-P", "full"]
}
~~~

Static files are served separately by Caddy from `/opt/trymosml/web` in
production. The old `Rootdir` option is ignored. For local development, configure
Caddy's root to the checkout's `web` directory and proxy `/webshell` and
`/healthz` to the Go server, following `deploy/Caddyfile`.
