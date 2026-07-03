/*
Package cli turns a scaffold service or stack into a small command-line
application.

The package intentionally accepts the core scaffold.Service interface.
Plain services can be started, stopped, and used with the generated
commands. Stacks expose richer commands because they can provide
environment variables, endpoints, summaries, running-state discovery,
and Docker resource discovery.
*/
package cli
