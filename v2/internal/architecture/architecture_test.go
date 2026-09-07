package architecture

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

const modulePath = "github.com/opendatahub-io/odh-platform-utilities/v2"

var (
	errForbiddenV1Path    = errors.New("imports forbidden v1 path")
	errKubeDependency     = errors.New("violates kube/resources dependency direction")
	errPlatformDependency = errors.New("violates platform dependency direction")
	errConcreteAction     = errors.New("imports a concrete action package")
)

func TestDependencyDirection(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	root := moduleRoot(t)
	g.Expect(validateTree(root)).Should(Succeed())
}

func TestDependencyDirectionRejectsForbiddenImports(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	path := filepath.Join(t.TempDir(), "bad.go")
	source := "package resources\n" +
		"import _ \"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action\"\n"
	err := os.WriteFile(path, []byte(source), 0o600)
	g.Expect(err).Should(Succeed())
	g.Expect(validateFile("pkg/kube/resources/bad.go", path)).Should(MatchError(ContainSubstring("dependency direction")))
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine architecture test location")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func validateTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		return validateFile(filepath.ToSlash(relativePath), path)
	})
}

func validateFile(relativePath, path string) error {
	imports, err := fileImports(path)
	if err != nil {
		return err
	}

	err = validateImportPaths(relativePath, imports)
	if err != nil {
		return err
	}

	return validatePackageDependencies(relativePath, imports)
}

func fileImports(path string) ([]string, error) {
	fileSet := token.NewFileSet()

	file, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return nil, err
	}

	imports := make([]string, 0, len(file.Imports))
	for _, importSpec := range file.Imports {
		imports = append(imports, strings.Trim(importSpec.Path.Value, `"`))
	}

	return imports, nil
}

func validateImportPaths(relativePath string, imports []string) error {
	for _, importedPath := range imports {
		if strings.Contains(importedPath, "/framework") || strings.HasSuffix(importedPath, "/pkg") {
			return fmt.Errorf("%s: %w: %q", relativePath, errForbiddenV1Path, importedPath)
		}
	}

	return nil
}

func validatePackageDependencies(relativePath string, imports []string) error {
	packagePath := filepath.ToSlash(filepath.Dir(relativePath))
	if packagePath == "pkg/kube/resources" && importsForbidden(imports, "/pkg/action", "/pkg/controller") {
		return fmt.Errorf("%s: %w", relativePath, errKubeDependency)
	}

	if strings.HasPrefix(packagePath, "pkg/platform") && importsForbidden(imports, "/pkg/action", "/pkg/controller") {
		return fmt.Errorf("%s: %w", relativePath, errPlatformDependency)
	}

	if packagePath == "pkg/controller/pipeline" && importsForbidden(imports, "/pkg/action/") {
		return fmt.Errorf("%s: %w", relativePath, errConcreteAction)
	}

	return nil
}

func importsForbidden(imports []string, suffixes ...string) bool {
	for _, importedPath := range imports {
		if !strings.HasPrefix(importedPath, modulePath) {
			continue
		}

		for _, suffix := range suffixes {
			if strings.Contains(importedPath, suffix) {
				return true
			}
		}
	}

	return false
}
