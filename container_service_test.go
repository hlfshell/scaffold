package scaffold

import (
	"testing"

	scaffoldcontainer "github.com/hlfshell/scaffold/container"
)

func TestFromContainerUsesContainerName(t *testing.T) {
	container, err := scaffoldcontainer.NewContainer("web", "nginx")
	if err != nil {
		t.Fatal(err)
	}

	service, err := FromContainer(container)
	if err != nil {
		t.Fatal(err)
	}

	if service.Name() != "web" {
		t.Fatalf("expected service name web, got %s", service.Name())
	}
}

func TestFromContainerAllowsExplicitServiceName(t *testing.T) {
	container, err := scaffoldcontainer.NewContainer("", "nginx")
	if err != nil {
		t.Fatal(err)
	}

	service, err := FromContainer(container, WithName("web"))
	if err != nil {
		t.Fatal(err)
	}

	if service.Name() != "web" {
		t.Fatalf("expected service name web, got %s", service.Name())
	}
}

func TestFromContainerRequiresName(t *testing.T) {
	container, err := scaffoldcontainer.NewContainer("", "nginx")
	if err != nil {
		t.Fatal(err)
	}

	_, err = FromContainer(container)
	if err == nil {
		t.Fatal("expected error")
	}

	expected := "container service requires a name: pass a named container or WithName"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}
