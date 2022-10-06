package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"path"
	"regexp"
	"strings"

	"git.dolansoft.org/lorenz/winsysroot/cab"
	"git.dolansoft.org/lorenz/winsysroot/msi"
)

var includeRegexp = regexp.MustCompile(`^Windows Kits/[^/]+/Include/[0-9\.]+/.*\.h(pp)?$`)
var libRegexp = regexp.MustCompile(`^Windows Kits/[^/]+/Lib/[0-9\.]+/.*\.[Ll][Ii][Bb]`)

func buildWinSDK(version string, architectures []string, slim bool, manifest InstallerManifest, out TargetI) {
	hasArch := make(map[string]bool)
	for _, arch := range architectures {
		hasArch[arch] = true
	}
	packageRegexp := regexp.MustCompile(`^Win.*SDK_` + regexp.QuoteMeta(version) + "$")
	var sdkPkg Package
	for _, pkg := range manifest.Packages {
		if packageRegexp.MatchString(pkg.ID) {
			sdkPkg = pkg
			break
		}
	}
	if sdkPkg.ID == "" {
		log.Fatalf("Failed to find Windows SDK with specified version")
	}
	cabs := make(map[string]*msi.MSI)
	for _, payload := range sdkPkg.Payloads {
		if strings.HasSuffix(payload.FileName, ".msi") {
			res, err := handleHTTPError(http.Get(payload.URL))
			if err != nil {
				log.Fatalf("failed to download MSI %v: %v", payload.FileName, err)
			}
			msiRaw, err := io.ReadAll(res.Body)
			if err != nil {
				log.Fatalf("failed to read MSI %v: %v", payload.FileName, err)
			}
			res.Body.Close()
			msiData, err := msi.Parse(bytes.NewReader(msiRaw))
			if err != nil {
				log.Fatalf("failed to parse MSI %v: %v", payload.FileName, err)
			}
			for _, targetFile := range msiData.FileMap {
				if includeRegexp.MatchString(targetFile) || libRegexp.MatchString(targetFile) {
					for _, cab := range msiData.CABFiles {
						cabs[strings.ToLower(cab)] = msiData
					}
					break
				}
			}
		}
	}
	for _, payload := range sdkPkg.Payloads {
		parts := strings.Split(payload.FileName, "\\")
		if len(parts) != 2 {
			continue
		}
		msiInfo := cabs[strings.ToLower(parts[1])]
		if msiInfo != nil {
			res, err := handleHTTPError(http.Get(payload.URL))
			if err != nil {
				log.Fatalf("failed to download CAB %v: %v", payload.FileName, err)
			}
			cabRaw, err := io.ReadAll(res.Body)
			if err != nil {
				log.Fatalf("failed to read CAB %v: %v", payload.FileName, err)
			}
			res.Body.Close()
			cabF, err := cab.New(bytes.NewReader(cabRaw))
			if err != nil {
				log.Fatalf("Failed to read CAB file: %v", err)
			}
			for {
				hdr, err := cabF.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Fatalf("Failed to read CAB file %q: %v", payload.FileName, err)
				}
				outPath := msiInfo.FileMap[hdr.Name]
				if outPath == "" {
					log.Printf("Unknown file %q in CAB, ignoring", hdr.Name)
					continue
				}
				parts := strings.Split(outPath, "/")
				typeDir := strings.ToLower(parts[2])
				if typeDir == "include" {
					if slim {
						ext := strings.ToLower(path.Ext(outPath))
						if ext != "" && ext != ".h" && ext != ".hpp" && ext != ".c" && ext != ".cpp" {
							continue
						}
					}
				} else if typeDir == "lib" {
					archDir := strings.ToLower(parts[5])
					if !hasArch[archDir] {
						continue
					}
					if slim {
						ext := strings.ToLower(path.Ext(outPath))
						if ext != ".lib" && ext != ".obj" {
							continue
						}
					}
				} else {
					continue
				}
				if err := out.Create(outPath, int64(hdr.Size), hdr.CreateTime); err != nil {
					log.Fatalf("Failed to create output file: %v", err)
				}
				if _, err := io.Copy(out, cabF); err != nil {
					log.Fatalf("Failed to extract from cab: %v", err)
				}
			}
		}
	}
}
