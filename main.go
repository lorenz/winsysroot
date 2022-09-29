package main

import (
	"archive/tar"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

var (
	flagVSRelease     = flag.String("vs-release", "17", "Major release of Visual Studio to generate sysroot from (like 14, 17, ..)")
	flagWinSDKVersion = flag.String("win-sdk-version", "10.0.20348", "Version of the Windows SDK to use, without the patch version (e.g. 10.0.20348)")
	flagArchitectures = flag.String("architectures", "x64", "Comma-separated list of architectures to include in the sysroot. Supported are x86, x64, arm, arm64 and arm64ec.")
	flagSlim          = flag.Bool("slim", true, "Strip most excess files, ship only headers, libraries and object files. Also strips separate onecore, store and uwp libraries.")
)

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
	flag.Parse()

	architectures := strings.Split(*flagArchitectures, ",")

	res, err := handleHTTPError(http.Get("https://aka.ms/vs/" + *flagVSRelease + "/release/channel"))
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

	var vfs VFS
	vfs.Version = 1
	vfs.RedirectingWith = RedirectingWithRedirectOnly

	winsysRoot := Inode{
		Type: "directory",
		Name: "test",
	}
	vfs.Roots = append(vfs.Roots, &winsysRoot)

	buildWinSDK(*flagWinSDKVersion, architectures, *flagSlim, installerManifest, out, &winsysRoot)
	buildVCTools(installerManifest, architectures, *flagSlim, out, &winsysRoot)
	vfsRaw, err := json.MarshalIndent(&vfs, "", "\t")
	if err != nil {
		log.Fatalf("Failed to encode VFS overlay metadata: %v", err)
	}
	if err := out.WriteHeader(&tar.Header{
		Name:    "vfsoverlay.yaml",
		ModTime: time.Now(),
		Mode:    0644,
		Size:    int64(len(vfsRaw)),
	}); err != nil {
		log.Fatalf("Failed to write header for VFS overlay: %v", err)
	}
	if _, err := out.Write(vfsRaw); err != nil {
		log.Fatalf("Failed to write VFS overlay: %v", err)
	}
	if err := out.Close(); err != nil {
		log.Fatalf("failed to close tar writer: %v", err)
	}
}
