package container

import (
	"archive/tar"
	"io"
	"os"
	"strings"
	"testing"
)

func TestDockerBuildContextUsesDockerfileDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir+"/Dockerfile", "FROM scratch\n")
	writeTestFile(t, dir+"/app.txt", "hello\n")

	reader, dockerfile, err := dockerBuildContext(dir + "/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	if dockerfile != "Dockerfile" {
		t.Fatalf("expected Dockerfile, got %s", dockerfile)
	}

	files := map[string]string{}
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.FileInfo().IsDir() {
			continue
		}

		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatal(err)
		}
		files[header.Name] = string(content)
	}

	if files["Dockerfile"] != "FROM scratch\n" {
		t.Fatalf("dockerfile was not included: %#v", files)
	}
	if files["app.txt"] != "hello\n" {
		t.Fatalf("context file was not included: %#v", files)
	}
}

func TestReadDockerBuildOutputReturnsLogsAndImageID(t *testing.T) {
	output := strings.NewReader(`{"stream":"Step 1/1 : FROM scratch\n"}` + "\n" + `{"aux":{"ID":"sha256:abc"}}` + "\n")

	image, logs, err := readDockerBuildOutput(output)
	if err != nil {
		t.Fatal(err)
	}
	if image != "sha256:abc" {
		t.Fatalf("expected image id, got %s", image)
	}
	if !strings.Contains(logs, "Step 1/1") {
		t.Fatalf("expected build logs, got %q", logs)
	}
}

func TestReadDockerBuildOutputReturnsBuildErrorWithLogs(t *testing.T) {
	output := strings.NewReader(`{"stream":"Step 1/1 : RUN false\n"}` + "\n" + `{"error":"executor failed"}` + "\n")

	_, logs, err := readDockerBuildOutput(output)
	if err == nil {
		t.Fatal("expected error")
	}
	if logs == "" {
		t.Fatal("expected logs")
	}
	if !strings.Contains(logs, "executor failed") {
		t.Fatalf("expected docker error in logs, got %q", logs)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
