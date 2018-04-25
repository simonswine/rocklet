package navmap

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/lmittmann/ppm"
	"github.com/rs/zerolog"

	"github.com/simonswine/rocklet/pkg/api"
	"github.com/simonswine/rocklet/pkg/apis/vacuum/v1alpha1"
)

type NavMap struct {
	stopCh chan struct{}
	logger zerolog.Logger
	flags  *api.Flags
	wg     sync.WaitGroup

	latestMap     *Map
	latestMapLock sync.Mutex
}

type Map struct {
	mTime time.Time
	path  string
	image image.Image

	jpegBuf *bytes.Buffer
	pngBuf  *bytes.Buffer
}

func NewMap(path string, mTime time.Time) (*Map, error) {
	m := &Map{
		mTime: mTime,
		path:  path,
	}

	in, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	m.image, err = ppm.Decode(in)
	if err != nil {
		return nil, err
	}

	return m, nil

}

func (m *Map) handlePNG(w http.ResponseWriter, r *http.Request) {

	if m.pngBuf == nil {
		buffer := new(bytes.Buffer)
		if err := png.Encode(buffer, m.image); err != nil {
			http.Error(w, fmt.Sprintf("unable to encode image to png: %s", err), http.StatusInternalServerError)
			return
		}
		m.pngBuf = buffer
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(m.pngBuf.Bytes())))
	w.Write(m.pngBuf.Bytes())
}

func (m *Map) handleJPEG(w http.ResponseWriter, r *http.Request) {

	if m.jpegBuf == nil {
		buffer := new(bytes.Buffer)
		if err := jpeg.Encode(buffer, m.image, &jpeg.Options{Quality: 90}); err != nil {
			http.Error(w, fmt.Sprintf("unable to encode image to jpeg: %s", err), http.StatusInternalServerError)
			return
		}
		m.jpegBuf = buffer
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(m.jpegBuf.Bytes())))
	w.Write(m.jpegBuf.Bytes())
}

func New(flags *api.Flags) *NavMap {
	n := &NavMap{
		logger: zerolog.New(os.Stdout).With().
			Str("app", "navmap").
			Logger().Level(zerolog.DebugLevel),
		flags: flags,
	}

	return n
}

func (n *NavMap) Logger() *zerolog.Logger {
	return &n.logger
}

func (n *NavMap) SetupHandler(serveMux *http.ServeMux) {
	serveMux.HandleFunc("/navmap/jpeg", n.handleJPEG)
	serveMux.HandleFunc("/navmap/png", n.handlePNG)
}

func (n *NavMap) loopHTTPServer() {
	defer n.wg.Done()
	serveMux := http.NewServeMux()
	n.SetupHandler(serveMux)

	s := &http.Server{
		Addr:    ":1888",
		Handler: serveMux,
	}
	log.Fatal(s.ListenAndServe())
}

func (n *NavMap) LoopMaps(stopCh chan struct{}) {
	c := time.Tick(200 * time.Millisecond)

	lookup := func() {
		err := n.lookupMaps()
		if err != nil && !os.IsNotExist(err) {
			n.logger.Warn().Err(err).Msg("error looking up maps")
		}
	}
	lookup()

	for {
		select {
		case <-n.stopCh:
			return
		case <-c:
			lookup()
		}
	}
}

// this method looks for new maps
func (n *NavMap) lookupMaps() error {
	matches, err := filepath.Glob(filepath.Join(n.flags.RuntimeDirectory, "navmap*.ppm"))
	if err != nil {
		return err
	}

	var newestFile string
	var newestTime time.Time

	for _, path := range matches {
		stat, err := os.Stat(path)

		// error reading modified time
		if err != nil {
			n.logger.Warn().Err(err).Msg("error getting last modified time")
			continue
		}

		// file not newer
		if newestTime.After(stat.ModTime()) {
			continue
		}

		newestFile = path
		newestTime = stat.ModTime()
	}

	n.latestMapLock.Lock()
	defer n.latestMapLock.Unlock()

	// file already known
	if n.latestMap != nil && n.latestMap.path == newestFile && n.latestMap.mTime == newestTime {
		return nil
	}

	latestMap, err := NewMap(newestFile, newestTime)
	if err != nil {
		return err
	}

	n.latestMap = latestMap
	n.logger.Debug().Str("map", newestFile).Msg("updated latest map")

	return nil
}

func (n *NavMap) Run() error {

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.LoopMaps(n.stopCh)
	}()

	n.wg.Add(1)
	go n.loopHTTPServer()

	n.wg.Wait()
	return nil
}

func (n *NavMap) handlePNG(w http.ResponseWriter, r *http.Request) {
	m := n.latestMap
	if m == nil {
		http.Error(w, "no map available", http.StatusNotFound)
		return
	}

	m.handlePNG(w, r)

}

func (n *NavMap) handleJPEG(w http.ResponseWriter, r *http.Request) {
	m := n.latestMap
	if m == nil {
		http.Error(w, "no map available", http.StatusNotFound)
		return
	}

	m.handleJPEG(w, r)

}

func ConvertMapToJPEG(input io.Reader, output io.Writer) error {
	image, err := ppm.Decode(input)
	if err != nil {
		return err
	}

	if err := jpeg.Encode(output, image, &jpeg.Options{Quality: 90}); err != nil {
		return err
	}

	return nil
}

func (n *NavMap) WatchCleaning() (chan *v1alpha1.Cleaning, error) {
	return nil, fmt.Errorf("unimplemented")
}
