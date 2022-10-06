package main

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

var (
	flagVSRelease     = flag.String("vs-release", "17", "Major release of Visual Studio to generate sysroot from (like 14, 17, ..)")
	flagWinSDKVersion = flag.String("win-sdk-version", "10.0.20348", "Version of the Windows SDK to use, without the patch version (e.g. 10.0.20348)")
	flagArchitectures = flag.String("architectures", "x64", "Comma-separated list of architectures to include in the sysroot. Supported are x86, x64, arm, arm64 and arm64ec.")
	flagSlim          = flag.Bool("slim", true, "Strip most excess files, ship only headers, libraries and object files. Also strips separate onecore, store and uwp libraries.")
	flagOutDir        = flag.String("out-dir", "", "Output sysroot under this directory. Exclusive with --out-tar.")
	flagOutTar        = flag.String("out-tar", "", "Output sysroot to a zstd-compressed tarball at the path given to this argument. Exclusive with --out-dir.")
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

type TargetI interface {
	Create(path string, size int64, modTime time.Time) error
	io.WriteCloser
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

	var out TargetI

	if flagOutDir != nil && *flagOutDir != "" {
		out = newVFSTargetLayer(&directoryTarget{rootDir: *flagOutDir}, *flagOutDir)
	} else if flagOutTar != nil && *flagOutTar != "" {
		outInner, err := newArchiveTarget(*flagOutTar)
		if err != nil {
			log.Fatalf("Failed to create output tar archive: %v", err)
		}
		out = newVFSTargetLayer(outInner, "/winsysroot")
	} else {
		log.Fatalln("Please pass either --out-dir or --out-tar to this command.")
	}

	buildWinSDK(*flagWinSDKVersion, architectures, *flagSlim, installerManifest, out)
	buildVCTools(installerManifest, architectures, *flagSlim, out)

	if err := out.Close(); err != nil {
		log.Fatalf("failed to finish wrinting output: %v", err)
	}
}

type vfsTargetLayer struct {
	t TargetI
	i *Inode
	v VFS
}

func newVFSTargetLayer(t TargetI, sysrootPath string) *vfsTargetLayer {
	var vfs VFS
	vfs.Version = 0
	vfs.RedirectingWith = RedirectingWithFallthrough
	True := true
	False := false
	vfs.CaseSensitive = &False
	vfs.OverlayRelative = &True

	winsysRoot := Inode{
		Type: "directory",
		Name: sysrootPath,
	}
	vfs.Roots = append(vfs.Roots, &winsysRoot)
	return &vfsTargetLayer{
		t: t,
		i: &winsysRoot,
		v: vfs,
	}
}

func (v *vfsTargetLayer) Create(p string, size int64, modTime time.Time) error {
	if err := v.i.Place(path.Dir(p), true, &Inode{
		Type:             "file",
		Name:             path.Base(p),
		ExternalContents: p,
	}); err != nil {
		return err
	}
	return v.t.Create(p, size, modTime)
}

func (v *vfsTargetLayer) Write(b []byte) (int, error) {
	return v.t.Write(b)
}

func (v *vfsTargetLayer) Close() error {
	vfsRaw, err := json.MarshalIndent(v.v, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to encode VFS overlay metadata: %w", err)
	}
	v.t.Create("vfsoverlay.yaml", int64(len(vfsRaw)), time.Now())
	if _, err := v.t.Write(vfsRaw); err != nil {
		return fmt.Errorf("failed to write VFS overlay: %w", err)
	}
	return v.t.Close()
}

type archiveTarget struct {
	outFile *os.File
	outComp *zstd.Encoder
	out     *tar.Writer
}

func newArchiveTarget(name string) (*archiveTarget, error) {
	outFile, err := os.Create(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create output archive: %w", err)
	}
	outComp, err := zstd.NewWriter(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize zstd compressor: %w", err)
	}
	out := tar.NewWriter(outComp)
	return &archiveTarget{
		outFile: outFile,
		outComp: outComp,
		out:     out,
	}, nil
}

func (a *archiveTarget) Close() error {
	if err := a.out.Close(); err != nil {
		return err
	}
	if err := a.outComp.Close(); err != nil {
		return err
	}
	if err := a.outFile.Close(); err != nil {
		return err
	}
	return nil
}

func (a *archiveTarget) Create(path string, size int64, modTime time.Time) error {
	return a.out.WriteHeader(&tar.Header{
		Name:    path,
		ModTime: modTime,
		Size:    size,
		Mode:    0644,
	})
}

func (a *archiveTarget) Write(b []byte) (int, error) {
	return a.out.Write(b)
}

type directoryTarget struct {
	rootDir  string
	currFile *os.File
}

func (d *directoryTarget) Create(path string, size int64, modTime time.Time) error {
	if d.currFile != nil {
		d.currFile.Close()
	}
	targetPath := filepath.Join(d.rootDir, filepath.FromSlash(path))
	f, err := os.Create(targetPath)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		f, err = os.Create(targetPath)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	d.currFile = f
	return nil
}

func (d *directoryTarget) Write(b []byte) (int, error) {
	return d.currFile.Write(b)
}

func (d *directoryTarget) Close() error {
	if d.currFile != nil {
		return d.currFile.Close()
	}
	return nil
}
