package testutil

import (
	"testing"

	scaffoldcontainer "github.com/hlfshell/scaffold/container"
)

/*
RequireDocker skips the test when Docker is not available.
*/
func RequireDocker(t *testing.T) {
	t.Helper()

	if !scaffoldcontainer.DockerAvailable() {
		t.Skip("Docker is not available")
	}
}
