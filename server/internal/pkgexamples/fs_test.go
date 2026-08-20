package pkgexamples

import "testing"

func TestFilesStripSrcAndIncludeCI(t *testing.T) {
	files, err := Files(Go)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"hello.go", "hello_test.go", "go.mod", "README.md", ".xvpn-ci.sh"} {
		if _, ok := files[want]; !ok {
			t.Fatalf("falta %s", want)
		}
	}
	if _, ok := files["hello.go.src"]; ok {
		t.Fatal("não deveria expor .src")
	}
}
