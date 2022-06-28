package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
)

func buildVCTools(manifest InstallerManifest, out *tar.Writer) {
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
	chase(map[string]interface{}{
		"Microsoft.VisualStudio.Component.VC.Tools.ARM64":   true,
		"Microsoft.VisualStudio.Component.VC.Tools.x86.x64": true,
	})
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
			if strings.HasPrefix(file.Name, "Contents/VC/Tools/MSVC/") {
				err := out.WriteHeader(&tar.Header{
					Name: strings.TrimPrefix(file.Name, "Contents/"),
					Mode: 0644,
					Size: file.FileInfo().Size(),
				})
				if err != nil {
					panic(err)
				}
				f, err := file.Open()
				if err != nil {
					panic(err)
				}
				if _, err := io.Copy(out, f); err != nil {
					panic(err)
				}
				f.Close()
			}
		}
	}
	if err := out.Close(); err != nil {
		log.Fatalf("failed to close tar writer: %v", err)
	}
}
