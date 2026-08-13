package architecture

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const module = "github/master_go_cqrs/"

type goPackage struct {
	ImportPath string
	Imports    []string
	Name       string
}

func TestDependencyBoundaries(t *testing.T) {
	packages := listPackages(t)
	byPath := make(map[string]goPackage, len(packages))
	var violations []string
	for _, pkg := range packages {
		byPath[pkg.ImportPath] = pkg
		for _, dependency := range pkg.Imports {
			if violation := dependencyViolation(pkg.ImportPath, dependency); violation != "" {
				violations = append(violations, violation)
			}
		}
	}

	reachable := map[string]bool{}
	var visit func(string)
	visit = func(path string) {
		if reachable[path] {
			return
		}
		reachable[path] = true
		for _, dependency := range byPath[path].Imports {
			visit(dependency)
		}
	}
	for path, pkg := range byPath {
		if strings.HasPrefix(path, module+"cmd/") && pkg.Name == "main" {
			visit(path)
		}
	}
	for path := range byPath {
		if strings.HasPrefix(path, module+"services/") && !reachable[path] {
			violations = append(violations, path+" is not reachable from any cmd binary")
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("architecture violations:\n%s", strings.Join(violations, "\n"))
	}
}

func dependencyViolation(importer, dependency string) string {
	if !strings.HasPrefix(importer, module) || !strings.HasPrefix(dependency, module) {
		return ""
	}
	localImporter := strings.TrimPrefix(importer, module)
	localDependency := strings.TrimPrefix(dependency, module)

	if strings.HasPrefix(localImporter, "internal/platform/") || strings.HasPrefix(localImporter, "contracts/") {
		if strings.HasPrefix(localDependency, "services/") {
			return importer + " must not import " + dependency
		}
	}
	if strings.HasPrefix(localImporter, "services/") && strings.HasPrefix(localDependency, "services/") {
		owner := serviceOwner(localImporter)
		if dependencyOwner := serviceOwner(localDependency); dependencyOwner != owner {
			return importer + " must not import service " + dependencyOwner
		}
	}
	if strings.Contains(localImporter, "/internal/domain") {
		if strings.Contains(localDependency, "/internal/application") || strings.Contains(localDependency, "/internal/adapter") || strings.Contains(localDependency, "/internal/transport") || strings.HasPrefix(localDependency, "internal/platform/") {
			return importer + " domain must not import " + dependency
		}
	}
	if strings.Contains(localImporter, "/internal/application/") {
		if strings.Contains(localDependency, "/internal/adapter/") || strings.Contains(localDependency, "/internal/transport/") {
			return importer + " application must not import " + dependency
		}
	}
	if strings.Contains(localImporter, "/internal/transport/") && strings.Contains(localDependency, "/internal/adapter/") {
		return importer + " transport must depend on application ports, not " + dependency
	}
	if strings.HasPrefix(localImporter, "cmd/") && strings.HasPrefix(localDependency, "services/") {
		parts := strings.Split(localDependency, "/")
		if len(parts) != 2 || !strings.HasSuffix(localImporter, "/"+parts[1]) {
			return importer + " may only import its service facade, got " + dependency
		}
	}
	return ""
}

func serviceOwner(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func listPackages(t *testing.T) []goPackage {
	t.Helper()
	root := repositoryRoot(t)
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []goPackage
	for {
		var pkg goPackage
		if err := decoder.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		packages = append(packages, pkg)
	}
	return packages
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
