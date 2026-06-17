// Package prismget downloads and unpacks a self-contained, portable Prism
// Launcher when no launcher is present on the machine. Nothing is installed
// system-wide: the official "Portable" build is extracted into a single folder
// and switched to portable mode (a portable.txt next to the binary), so Prism
// keeps its data — instances, accounts, config — inside that same folder.
// Removing Prism is then just deleting the folder.
package prismget

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"modpack-installer/internal/httpx"
	"modpack-installer/internal/mcpaths"
)

const defaultRepo = "PrismLauncher/PrismLauncher"

// maxExtract caps the total decompressed size, so a malicious or corrupt archive
// cannot fill the disk. The real portable builds are ~150–200 MB unpacked.
const maxExtract = 1 << 30 // 1 GB

// Result describes the Prism we ensured is present.
type Result struct {
	InstancesDir string // where the modpack instance must be written
	LaunchPath   string // the file the user opens to start Prism
	Version      string // release tag, when known
	Installed    bool   // true if we just downloaded it, false if it was already there
}

// EnsurePortable makes sure a portable Prism exists and returns where to write
// the instance plus how to launch it. It is idempotent: a second call reuses the
// already-extracted copy instead of downloading again.
//
// repo/version/overrideURL come from config (all optional). On dry-run it only
// reports the paths it would use, touching nothing.
func EnsurePortable(repo, version, overrideURL string, dryRun bool, logf func(string, ...any)) (*Result, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if repo == "" {
		repo = defaultRepo
	}
	goos, goarch := runtime.GOOS, runtime.GOARCH
	dest := mcpaths.PortablePrismDir()

	// Already extracted? Reuse it.
	if bin := findLauncher(dest, goos); bin != "" {
		logf("Portable Prism already present at %s", dest)
		return finalize(dest, bin, goos, version, false), nil
	}

	if dryRun {
		logf("[dry-run] would download & install a portable Prism Launcher into %s", dest)
		return &Result{
			InstancesDir: predictInstances(dest, goos),
			LaunchPath:   filepath.Join(dest, launcherBase(goos)),
			Version:      version,
		}, nil
	}

	// Resolve the asset URL: explicit override, or the right asset of a release.
	name, url := "", overrideURL
	if url != "" {
		name = path.Base(url)
		logf("Using configured Prism asset: %s", name)
	} else {
		rel, err := fetchRelease(repo, version)
		if err != nil {
			return nil, err
		}
		a, err := pickAsset(rel.Assets, goos, goarch)
		if err != nil {
			return nil, err
		}
		name, url, version = a.Name, a.URL, rel.TagName
		logf("Prism %s -> %s", rel.TagName, name)
	}

	logf("Downloading %s …", name)
	b, err := httpx.Bytes(url)
	if err != nil {
		return nil, fmt.Errorf("downloading Prism: %w", err)
	}
	logf("  %d MB", len(b)/1024/1024)

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	if err := extract(name, b, dest); err != nil {
		return nil, fmt.Errorf("extracting Prism: %w", err)
	}

	bin := findLauncher(dest, goos)
	if bin == "" {
		return nil, fmt.Errorf("extracted Prism but could not find the launcher binary under %s", dest)
	}
	if goos != "windows" {
		_ = os.Chmod(bin, 0o755)
	}
	return finalize(dest, bin, goos, version, true), nil
}

// finalize computes the instance dir and (on Linux/Windows) flips Prism into
// portable mode so its data lives next to the binary.
func finalize(dest, bin, goos, version string, installed bool) *Result {
	r := &Result{LaunchPath: bin, Version: version, Installed: installed}
	if goos == "darwin" {
		// macOS apps can't easily run portable; use the standard data dir Prism
		// would pick on its own (~/Library/Application Support/PrismLauncher).
		r.InstancesDir = mcpaths.PrismInstancesDir()
		return r
	}
	dataDir := filepath.Dir(bin)
	marker := filepath.Join(dataDir, "portable.txt")
	if _, err := os.Stat(marker); err != nil {
		_ = os.WriteFile(marker, []byte{}, 0o644)
	}
	r.InstancesDir = filepath.Join(dataDir, "instances")
	return r
}

func predictInstances(dest, goos string) string {
	if goos == "darwin" {
		return mcpaths.PrismInstancesDir()
	}
	return filepath.Join(dest, "instances")
}

func launcherBase(goos string) string {
	if goos == "windows" {
		return "prismlauncher.exe"
	}
	if goos == "darwin" {
		return "PrismLauncher.app"
	}
	return "PrismLauncher"
}

// findLauncher walks the extracted tree and returns the launcher path, or "".
func findLauncher(root, goos string) string {
	var hit string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || hit != "" {
			return nil
		}
		name := strings.ToLower(d.Name())
		if goos == "darwin" {
			if d.IsDir() && name == "prismlauncher.app" {
				hit = p
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if name == "prismlauncher" || name == "prismlauncher.exe" {
			hit = p
		}
		return nil
	})
	return hit
}

// --- GitHub release resolution -------------------------------------------------

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []ghAsset
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func fetchRelease(repo, version string) (*ghRelease, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	if version != "" {
		u = fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, version)
	}
	var rel ghRelease
	if err := httpx.JSON(u, &rel); err != nil {
		return nil, fmt.Errorf("resolving Prism release: %w", err)
	}
	return &rel, nil
}

// pickAsset selects the portable build for this OS/arch by matching name tokens.
func pickAsset(assets []ghAsset, goos, goarch string) (ghAsset, error) {
	var must, mustNot []string
	switch goos {
	case "linux":
		// e.g. PrismLauncher-Linux-Qt6-Portable-X.tar.gz   (x86_64, no arch token)
		//      PrismLauncher-Linux-aarch64-Qt6-Portable-X.tar.gz
		must = []string{"linux", "portable", ".tar.gz"}
		if goarch == "arm64" {
			must = append(must, "aarch64")
		} else {
			mustNot = []string{"aarch64"}
		}
	case "windows":
		// MinGW builds are self-contained (no MSVC redistributable needed).
		must = []string{"windows", "mingw", "portable", ".zip"}
		if goarch == "arm64" {
			must = append(must, "arm64")
		} else {
			must = append(must, "w64")
		}
	case "darwin":
		must = []string{"macos", ".zip"} // universal .app bundle
	default:
		return ghAsset{}, fmt.Errorf("unsupported OS for Prism auto-install: %s", goos)
	}
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if containsAll(n, must) && containsNone(n, mustNot) {
			return a, nil
		}
	}
	return ghAsset{}, fmt.Errorf("no portable Prism asset matched for %s/%s", goos, goarch)
}

func containsAll(s string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(s, t) {
			return false
		}
	}
	return true
}

func containsNone(s string, tokens []string) bool {
	for _, t := range tokens {
		if strings.Contains(s, t) {
			return false
		}
	}
	return true
}

// --- extraction ----------------------------------------------------------------

func extract(name string, data []byte, dest string) error {
	low := strings.ToLower(name)
	switch {
	case strings.HasSuffix(low, ".zip"):
		return unzip(data, dest)
	case strings.HasSuffix(low, ".tar.gz"), strings.HasSuffix(low, ".tgz"):
		return untargz(data, dest)
	default:
		return fmt.Errorf("unknown Prism archive type: %s", name)
	}
}

// safeJoin joins name onto dest and rejects anything that escapes dest (zip-slip).
func safeJoin(dest, name string) (string, error) {
	clean := filepath.Clean(filepath.Join(dest, name))
	if clean != dest && !strings.HasPrefix(clean, dest+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe path in archive: %q", name)
	}
	return clean, nil
}

func untargz(data []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	budget := int64(maxExtract)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(target, tr, os.FileMode(hdr.Mode)&0o777, &budget); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := writeSymlink(dest, target, hdr.Linkname); err != nil {
				return err
			}
		case tar.TypeLink:
			// Hard link: Linkname is the path of an already-extracted file,
			// relative to the archive root (Prism's portable build uses these
			// for its launcher entrypoints).
			if err := writeHardlink(dest, target, hdr.Linkname); err != nil {
				return err
			}
		}
	}
	return nil
}

func unzip(data []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	budget := int64(maxExtract)
	for _, f := range zr.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		mode := f.Mode()
		if mode&os.ModeSymlink != 0 {
			link, rerr := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if rerr != nil {
				return rerr
			}
			if err := writeSymlink(dest, target, string(link)); err != nil {
				return err
			}
			continue
		}
		perm := mode.Perm()
		if perm == 0 {
			perm = 0o644
		}
		err = writeFile(target, rc, perm, &budget)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFile(target string, src io.Reader, mode os.FileMode, budget *int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	n, err := io.Copy(out, io.LimitReader(src, *budget+1))
	*budget -= n
	if *budget < 0 {
		return fmt.Errorf("archive exceeds the %d-byte extraction limit", int64(maxExtract))
	}
	return err
}

func writeHardlink(dest, target, linkname string) error {
	src, err := safeJoin(dest, linkname)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	_ = os.Remove(target)
	return os.Link(src, target)
}

func writeSymlink(dest, target, link string) error {
	if filepath.IsAbs(link) {
		return fmt.Errorf("unsafe absolute symlink -> %q", link)
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(target), link))
	if resolved != dest && !strings.HasPrefix(resolved, dest+string(os.PathSeparator)) {
		return fmt.Errorf("symlink escapes destination: %q", link)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	_ = os.Remove(target)
	return os.Symlink(link, target)
}
