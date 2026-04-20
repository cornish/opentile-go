// download fetches openslide's public test slides into OPENTILE_TESTDIR. It is
// run manually (not by `go test`) to prepare a development machine.
//
// Usage:
//     OPENTILE_TESTDIR=$PWD/testdata/slides go run ./tests/download -slide CMU-1-Small-Region
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const openslideBase = "https://openslide.cs.cmu.edu/download/openslide-testdata"

type slide struct {
	name     string
	subpath  string // subpath under openslide-testdata/
	filename string
}

var knownSlides = map[string]slide{
	"CMU-1-Small-Region": {
		name:     "CMU-1-Small-Region",
		subpath:  "Aperio",
		filename: "CMU-1-Small-Region.svs",
	},
	// Additional slides can be added here when needed.
}

func main() {
	slideFlag := flag.String("slide", "CMU-1-Small-Region", "slide to download")
	flag.Parse()

	dest := os.Getenv("OPENTILE_TESTDIR")
	if dest == "" {
		log.Fatal("OPENTILE_TESTDIR not set; refusing to guess a destination")
	}
	s, ok := knownSlides[*slideFlag]
	if !ok {
		log.Fatalf("unknown slide: %q", *slideFlag)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", dest, err)
	}
	url := fmt.Sprintf("%s/%s/%s", openslideBase, s.subpath, s.filename)
	outPath := filepath.Join(dest, s.filename)
	if _, err := os.Stat(outPath); err == nil {
		log.Printf("already present: %s", outPath)
		return
	}
	log.Printf("downloading %s → %s", url, outPath)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("GET %s: %s", url, resp.Status)
	}
	out, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create %s: %v", outPath, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		log.Fatalf("download body: %v", err)
	}
	log.Printf("done: %s", outPath)
}
