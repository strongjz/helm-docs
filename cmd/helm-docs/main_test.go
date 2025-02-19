package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/norwoodj/helm-docs/pkg/document"
)

// BenchmarkHelmDocs benchmarks the entire helm-docs command by running on testdata.
//
// To run benchmarks, run the command:
//
//   go test -run=^$ -bench=. ./cmd/helm-docs
//
func BenchmarkHelmDocs(b *testing.B) {
	// Copy testdata to a new temporary directory, to keep the working directory clean.
	tmp := copyToTempDir(b, os.DirFS(filepath.Join("testdata", "benchmark")))

	// Bind commandline flags.
	// NOTE: Flags must be specified even if they use the default value.
	if err := viper.BindFlagValues(testFlagSet{
		"chart-search-root":          tmp,
		"log-level":                  "warn",
		"ignore-file":                ".helmdocsignore",
		"output-file":                "README.md",
		"sort-values-order":          document.AlphaNumSortOrder,
		"document-dependency-values": true,
	}); err != nil {
		b.Fatal(err)
	}

	// Benchmark the main function.
	for n := 0; n < b.N; n++ {
		helmDocs(nil, nil)
	}
}

var _ viper.FlagValueSet = &testFlagSet{}

type testFlagSet map[string]interface{}

func (s testFlagSet) VisitAll(fn func(viper.FlagValue)) {
	for k, v := range s {
		flagVal := &testFlagValue{
			name:  k,
			value: fmt.Sprintf("%v", v),
		}
		switch v.(type) {
		case bool:
			flagVal.typ = "bool"
		default:
			flagVal.typ = "string"
		}
		fn(flagVal)
	}
}

var _ viper.FlagValue = &testFlagValue{}

type testFlagValue struct {
	name  string
	value string
	typ   string
}

func (v *testFlagValue) HasChanged() bool {
	return false
}

func (v *testFlagValue) Name() string {
	return v.name
}

func (v *testFlagValue) ValueString() string {
	return v.value
}

func (v *testFlagValue) ValueType() string {
	return v.typ
}

// copyToTempDir copies the specified readonly filesystem into a new temporary directory and returns
// the path to the temporary directory. It fails the benchmark on any error and handles cleanup when
// the benchmark finishes.
func copyToTempDir(b *testing.B, fsys fs.FS) string {
	// Create the temporary directory.
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		b.Fatal(err)
	}

	// Register a cleanup function on the benchmark to clean up the temporary directory.
	b.Cleanup(func() {
		if err := os.RemoveAll(tmp); err != nil {
			b.Fatal(err)
		}
	})

	// Copy the filesystem to the temporary directory.
	if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, _ error) error {
		// Get the path info (contains permissions, etc.)
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Calculate the target path in the temporary directory.
		targetPath := filepath.Join(tmp, path)

		// If the path is a directory, create it.
		if d.IsDir() {
			if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
				return err
			}
			return nil
		}

		// If the path is a file, open it for reading.
		readFile, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer readFile.Close()

		// Open a new file in the temporary directory for writing.
		writeFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer writeFile.Close()

		// Copy the file.
		if _, err := io.Copy(writeFile, readFile); err != nil {
			return err
		}

		return nil
	}); err != nil {
		b.Fatal(err)
	}

	return tmp
}
