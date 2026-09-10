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

const legacyModulePath = "github.com/opendatahub-io/odh-platform-utilities"

var (
	errForbiddenV1Path       = errors.New("imports forbidden v1 path")
	errKubeDependency        = errors.New("violates kube/resources dependency direction")
	errPlatformDependency    = errors.New("violates platform dependency direction")
	errConcreteAction        = errors.New("imports a concrete action package")
	errPipelineAdapter       = errors.New("pipeline imports belong in action *_pipeline.go adapters")
	errOptionDependency      = errors.New("pkg/option must be dependency-free")
	errAPIDependency         = errors.New("api must not import v2 implementation packages")
	errControllerRootFile    = errors.New("pkg/controller is a package parent")
	errRendererPlacement     = errors.New("rendering belongs to manifest-kit or controller-owned actions")
	errManagerClientCoupling = errors.New("actions and pipeline must not depend on manager/client wrappers")
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

func TestDependencyDirectionRejectsPipelineImportsOutsideAdapters(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	path := filepath.Join(t.TempDir(), "action.go")
	source := "package action\n" +
		"import _ \"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline\"\n"
	err := os.WriteFile(path, []byte(source), 0o600)
	g.Expect(err).Should(Succeed())
	g.Expect(validateFile("pkg/action/example/action.go", path)).Should(MatchError(ContainSubstring("pipeline imports")))
}

func TestDependencyDirectionRejectsOptionDependencies(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	path := filepath.Join(t.TempDir(), "option.go")
	source := "package option\n" +
		"import _ \"fmt\"\n"
	err := os.WriteFile(path, []byte(source), 0o600)
	g.Expect(err).Should(Succeed())
	g.Expect(validateFile("pkg/option/option.go", path)).Should(MatchError(ContainSubstring("dependency-free")))
}

func TestDependencyDirectionRejectsAPIV2ImplementationImports(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	path := filepath.Join(t.TempDir(), "api.go")
	source := "package api\n" +
		"import _ \"" + modulePath + "/pkg/kube/resources\"\n"
	err := os.WriteFile(path, []byte(source), 0o600)
	g.Expect(err).Should(Succeed())
	g.Expect(validateFile("api/api.go", path)).Should(MatchError(ContainSubstring("implementation packages")))
}

func TestDependencyDirectionRejectsControllerRootFiles(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	path := filepath.Join(t.TempDir(), "controller.go")
	source := "package controller\n"
	err := os.WriteFile(path, []byte(source), 0o600)
	g.Expect(err).Should(Succeed())
	g.Expect(validateFile("pkg/controller/controller.go", path)).Should(MatchError(ContainSubstring("package parent")))
}

func TestDependencyDirectionRejectsRendererPackage(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	path := filepath.Join(t.TempDir(), "render.go")
	err := os.WriteFile(path, []byte("package render\n"), 0o600)
	g.Expect(err).Should(Succeed())
	err = validateFile("pkg/render/render.go", path)
	g.Expect(err).Should(MatchError(ContainSubstring("manifest-kit")))
}

func TestDependencyDirectionRejectsManagerClientCoupling(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	path := filepath.Join(t.TempDir(), "action.go")
	source := "package action\n" +
		"import _ \"" + modulePath + "/pkg/manager\"\n"
	err := os.WriteFile(path, []byte(source), 0o600)
	g.Expect(err).Should(Succeed())
	g.Expect(validateFile("pkg/action/example/action.go", path)).Should(MatchError(ContainSubstring("manager/client wrappers")))
}

func TestValidateImportPathsClassifiesModuleBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		imported  string
		forbidden bool
	}{
		{
			name:     "manifest kit package is allowed",
			imported: "github.com/k8s-manifest-kit/engine/pkg",
		},
		{
			name:      "legacy package is forbidden",
			imported:  legacyModulePath + "/pkg/resources",
			forbidden: true,
		},
		{
			name:      "legacy framework is forbidden",
			imported:  legacyModulePath + "/framework/controller",
			forbidden: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			err := validateImportPaths("pkg/example.go", []string{test.imported})

			if test.forbidden {
				g.Expect(err).Should(MatchError(ContainSubstring("forbidden v1 path")))
				return
			}

			g.Expect(err).Should(Succeed())
		})
	}
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
	packagePath := filepath.ToSlash(filepath.Dir(relativePath))

	for _, importedPath := range imports {
		if packagePath == "pkg/option" {
			return fmt.Errorf("%s: %w", relativePath, errOptionDependency)
		}

		if packagePath == "api" && strings.HasPrefix(importedPath, modulePath+"/pkg/") {
			return fmt.Errorf("%s: %w: %q", relativePath, errAPIDependency, importedPath)
		}

		if strings.HasPrefix(filepath.ToSlash(relativePath), "pkg/action/") &&
			importedPath == modulePath+"/pkg/controller/pipeline" &&
			!strings.HasSuffix(relativePath, "_pipeline.go") {
			return fmt.Errorf("%s: %w", relativePath, errPipelineAdapter)
		}

		if (strings.HasPrefix(packagePath, "pkg/action/") || packagePath == "pkg/controller/pipeline") &&
			(strings.HasPrefix(importedPath, modulePath+"/pkg/manager") ||
				strings.HasPrefix(importedPath, modulePath+"/pkg/client")) {
			return fmt.Errorf("%s: %w: %q", relativePath, errManagerClientCoupling, importedPath)
		}

		if strings.HasPrefix(importedPath, legacyModulePath+"/framework") ||
			importedPath == legacyModulePath+"/pkg" ||
			strings.HasPrefix(importedPath, legacyModulePath+"/pkg/") {
			return fmt.Errorf("%s: %w: %q", relativePath, errForbiddenV1Path, importedPath)
		}
	}

	return nil
}

func validatePackageDependencies(relativePath string, imports []string) error {
	packagePath := filepath.ToSlash(filepath.Dir(relativePath))
	if packagePath == "pkg/controller" {
		return fmt.Errorf("%s: %w", relativePath, errControllerRootFile)
	}

	if packagePath == "pkg/render" || strings.HasPrefix(packagePath, "pkg/render/") {
		return fmt.Errorf("%s: %w", relativePath, errRendererPlacement)
	}
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
