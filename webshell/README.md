Webshell
========

Custom webservice for Try Moscow ML.

Written in Go, the structure of the code is:

* `main.go` sets up an HTTP server that serves static content from a
  directory, and starts websocket sessions.
  
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
