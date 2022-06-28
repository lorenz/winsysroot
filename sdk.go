package main

import (
	"archive/tar"
	"bytes"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"git.dolansoft.org/lorenz/winsysroot/cab"
	"git.dolansoft.org/lorenz/winsysroot/msi"
)

var includeRegexp = regexp.MustCompile(`^Windows Kits/[^/]+/Include/[0-9\.]+/.*\.h(pp)?$`)
var libRegexp = regexp.MustCompile(`^Windows Kits/[^/]+/Lib/[0-9\.]+/.*\.[Ll][Ii][Bb]`)

func buildWinSDK(manifest InstallerManifest, out *tar.Writer) {
	var sdkPkg Package
	for _, pkg := range manifest.Packages {
		if pkg.ID == "Win10SDK_10.0.20348" {
			sdkPkg = pkg
			break
		}
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
				out.WriteHeader(&tar.Header{
					Name:    outPath,
					ModTime: hdr.CreateTime,
					Size:    int64(hdr.Size),
					Mode:    0644,
				})

				if _, err := io.Copy(out, cabF); err != nil {
					log.Fatalf("Failed to extract from cab: %v", err)
				}
			}
		}
	}

}
