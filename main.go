package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/klauspost/compress/zstd"
)

const channelManifestURL = "https://aka.ms/vs/17/release/channel"

func handleHTTPError(res *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		errorMsg, err := ioutil.ReadAll(io.LimitReader(res.Body, 1024))
		if err != nil {
			return nil, fmt.Errorf("HTTP %d: %w", res.StatusCode, err)
		}
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, string(errorMsg))
	}
	return res, nil
}

func main() {
	res, err := handleHTTPError(http.Get(channelManifestURL))
	if err != nil {
		log.Fatalf("failed to get channel manifest: %v", err)
	}
	var channel ChannelManifest
	if err := json.NewDecoder(res.Body).Decode(&channel); err != nil {
		log.Fatalf("failed to parse channel manifest: %v", err)
	}
	res.Body.Close()
	log.Printf("Using channel manifest %v", channel.Info.ID)
	var installerManifestURL string
	for _, item := range channel.ChannelItems {
		if item.ID == "Microsoft.VisualStudio.Manifests.VisualStudio" {
			installerManifestURL = item.Payloads[0].URL
		}
	}
	if installerManifestURL == "" {
		log.Fatalf("could not find installer manifest in channel manifest")
	}
	res, err = handleHTTPError(http.Get(installerManifestURL))
	if err != nil {
		log.Fatalf("failed to get installer manifest: %v", err)
	}
	var installerManifest InstallerManifest
	if err := json.NewDecoder(res.Body).Decode(&installerManifest); err != nil {
		log.Fatalf("failed to parse installer manifest: %v", err)
	}
	res.Body.Close()

	outFile, err := os.Create("winsysroot.tar.zst")
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer outFile.Close()
	outComp, err := zstd.NewWriter(outFile)
	if err != nil {
		log.Fatalf("Failed to initialize zstd compressor: %v", err)
	}
	defer outComp.Close()
	out := tar.NewWriter(outComp)
	defer out.Close()

	buildWinSDK(installerManifest, out)
	buildVCTools(installerManifest, out)
}
