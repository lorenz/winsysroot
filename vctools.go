package main

import (
	"archive/zip"
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
)

var archTools = map[string]string{
	"arm":     "Microsoft.VisualStudio.Component.VC.Tools.ARM",
	"arm64":   "Microsoft.VisualStudio.Component.VC.Tools.ARM64",
	"arm64ec": "Microsoft.VisualStudio.Component.VC.Tools.ARM64EC",
	"x64":     "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
	"x86":     "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
}

func buildVCTools(manifest InstallerManifest, architectures []string, slim bool, out TargetI) {
	pkgs := make(map[string]Package)
	var chase func(ids map[string]interface{})
	chase = func(ids map[string]interface{}) {
		for _, pkg := range manifest.Packages {
			if _, ok := ids[pkg.ID]; !ok {
				continue
			}
			if _, ok := pkgs[pkg.ID]; ok {
				continue
			}
			pkgs[pkg.ID] = pkg
			if len(pkg.Dependencies) > 0 {
				chase(pkg.Dependencies)
			}
		}
	}
	hasArch := make(map[string]bool)
	roots := make(map[string]interface{})
	for _, arch := range architectures {
		component := archTools[arch]
		if component == "" {
			log.Fatalf("unknown architecture %q, don't know the correct tools package", arch)
		}
		roots[component] = true
		hasArch[arch] = true
	}
	chase(roots)
	log.Printf("Downloading %d packages", len(pkgs))
	for _, pkg := range pkgs {
		if !strings.EqualFold(pkg.Type, "vsix") {
			continue
		}
		log.Printf("Downloading %s %s", pkg.ID, pkg.Version)
		res, err := handleHTTPError(http.Get(pkg.Payloads[0].URL))
		if err != nil {
			log.Fatalf("failed to download package %v: %v", pkg.ID, err)
		}
		payload, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatalf("failed to read package %v: %v", pkg.ID, err)
		}
		res.Body.Close()
		archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
		for _, file := range archive.File {
			if !strings.HasPrefix(file.Name, "Contents/VC/Tools/MSVC/") {
				continue
			}
			parts := strings.Split(file.Name, "/")
			typeDir := strings.ToLower(parts[5])
			if typeDir != "include" && typeDir != "lib" {
				continue
			}
			if typeDir == "lib" && !hasArch[strings.ToLower(parts[6])] {
				continue
			}
			targetPath := strings.TrimPrefix(file.Name, "Contents/")
			if err := out.Create(targetPath, file.FileInfo().Size(), file.FileInfo().ModTime()); err != nil {
				log.Fatalf("Failed to create output file: %v", err)
			}
			f, err := file.Open()
			if err != nil {
				log.Fatalf("Package %q: failed to open file %q: %v", pkg.ID, file.Name, err)
			}
			if _, err := io.Copy(out, f); err != nil {
				log.Fatalf("Package %q: failed to copy file %q to target: %v", pkg.ID, file.Name, err)
			}
			f.Close()
		}
	}
}
