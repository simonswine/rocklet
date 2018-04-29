package navmap

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
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

	// these are full map pixel positions, full is 1024x1024
	positions     []v1alpha1.Position
	positionsLock sync.Mutex
}

type Map struct {
	mTime time.Time
	path  string
	image image.Image

	jpegBuf *bytes.Buffer
	pngBuf  *bytes.Buffer

	charger       *v1alpha1.Position
	kubernetesMap *v1alpha1.Map
}

type imageChangeable interface {
	image.Image
	Set(x, y int, c color.Color)
	SubImage(r image.Rectangle) image.Image
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

	cimg, ok := m.image.(imageChangeable)
	if !ok {
		return nil, fmt.Errorf("unable to change image")
	}

	// record used area
	bounds := cimg.Bounds()
	minX := bounds.Max.X
	maxX := bounds.Min.X
	minY := bounds.Max.Y
	maxY := bounds.Min.Y

	updateMinMax := func(x int, y int) {
		if minX > x {
			minX = x
		}
		if minY > y {
			minY = y
		}
		if maxX < x {
			maxX = x
		}
		if maxY < y {
			maxY = y
		}
	}

	// recolorize image
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := cimg.At(x, y).RGBA()
			c1 := color.RGBA{R: 125, G: 125, B: 125, A: 255}
			c2 := color.RGBA{R: 255, G: 255, B: 255, A: 255}
			c3 := color.RGBA{R: 0, G: 0, B: 0, A: 255}
			c4 := color.RGBA{R: 0xfa, G: 0xfa, B: 0xfa, A: 0xff}

			if cR, cG, cB, cA := c1.RGBA(); cR == r && cG == g && cB == b && cA == a {
				cimg.Set(x, y, colorUnknown)
			} else if cR, cG, cB, cA := c2.RGBA(); cR == r && cG == g && cB == b && cA == a {
				cimg.Set(x, y, colorInside)
				updateMinMax(x, y)
			} else if cR, cG, cB, cA := c3.RGBA(); cR == r && cG == g && cB == b && cA == a {
				cimg.Set(x, y, colorWall)
				updateMinMax(x, y)
			} else if cR, cG, cB, cA := c4.RGBA(); cR == r && cG == g && cB == b && cA == a {
				// charger
				m.charger = &v1alpha1.Position{
					X: float32(x),
					Y: float32(y),
				}
				updateMinMax(x, y)
			} else {
				updateMinMax(x, y)
			}
		}
	}

	m.image = cimg.SubImage(image.Rectangle{
		Min: image.Point{X: minX, Y: minY},
		Max: image.Point{X: maxX, Y: maxY},
	})

	buffer := new(bytes.Buffer)
	if err := png.Encode(buffer, m.image); err != nil {
		return nil, err
	}

	m.kubernetesMap = &v1alpha1.Map{
		Left:   uint32(minX),
		Top:    uint32(bounds.Max.Y - maxY),
		Width:  uint32(m.image.Bounds().Max.X),
		Height: uint32(m.image.Bounds().Max.Y),
		Data:   buffer.Bytes(),
	}

	return m, nil

}

func (m *Map) PNG() ([]byte, error) {
	if m.pngBuf == nil {
		buffer := new(bytes.Buffer)
		if err := png.Encode(buffer, m.image); err != nil {
			return nil, err
		}
		m.pngBuf = buffer
	}
	return m.pngBuf.Bytes(), nil
}

func (m *Map) handlePNG(w http.ResponseWriter, r *http.Request) {

	data, err := m.PNG()
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to encode image to png: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
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

	// start slam log watcher
	go n.watchSlamLog()

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

func (n *NavMap) Positions() []v1alpha1.Position {
	n.positionsLock.Lock()
	defer n.positionsLock.Unlock()
	return n.positions
}

func (n *NavMap) positionsEmpty() {
	n.positionsLock.Lock()
	defer n.positionsLock.Unlock()
	n.positions = []v1alpha1.Position{}
}

func (n *NavMap) positionsAppend(pos v1alpha1.Position) {
	n.positionsLock.Lock()
	defer n.positionsLock.Unlock()

	// skip entry if vacuum is stationary
	if len(n.positions) > 0 {
		lastPos := n.positions[len(n.positions)-1]
		if reflect.DeepEqual(lastPos, pos) {
			return
		}
	}

	// append position
	n.positions = append(
		n.positions,
		pos,
	)
}

func (n *NavMap) LatestPositionsMap() ([]v1alpha1.Position, *v1alpha1.Map, *v1alpha1.Position, error) {
	var pos []v1alpha1.Position

	n.latestMapLock.Lock()
	latestMap := n.latestMap
	n.latestMapLock.Unlock()

	if latestMap == nil {
		return pos, nil, nil, fmt.Errorf("no image found")
	}

	if latestMap.kubernetesMap == nil {
		return pos, nil, nil, fmt.Errorf("no kubernetes map found")
	}

	// convert coordinates for this image
	for _, p := range n.positions {
		pos = append(pos, v1alpha1.Position{
			X: p.X - float32(latestMap.kubernetesMap.Left),
			Y: p.Y - float32(latestMap.kubernetesMap.Top),
		})
	}

	return pos, latestMap.kubernetesMap, latestMap.charger, nil
}

func (n *NavMap) WatchCleaning() (chan *v1alpha1.Cleaning, error) {
	return nil, fmt.Errorf("unimplemented")
}

type slamLine struct {
	Command   string
	X         float64
	Y         float64
	Angle     float64
	Int       int
	Timestamp float64
}

type tailLogger struct {
	zerolog.Logger
}

func (t tailLogger) Fatal(v ...interface{}) {
	t.Logger.Fatal().Msg(fmt.Sprint(v...))
}

func (t tailLogger) Fatalf(fmt string, v ...interface{}) {
	t.Logger.Fatal().Msgf(fmt, v...)
}

func (t tailLogger) Fatalln(v ...interface{}) {
	t.Logger.Fatal().Msg(fmt.Sprint(v...))
}

func (t tailLogger) Print(v ...interface{}) {
	t.Logger.Info().Msg(fmt.Sprint(v...))
}

func (t tailLogger) Printf(fmt string, v ...interface{}) {
	t.Logger.Info().Msgf(fmt, v...)
}

func (t tailLogger) Println(v ...interface{}) {
	t.Logger.Info().Msg(fmt.Sprint(v...))
}
func (t tailLogger) Panic(v ...interface{}) {
	t.Logger.Fatal().Msg(fmt.Sprint(v...))
}

func (t tailLogger) Panicf(fmt string, v ...interface{}) {
	t.Logger.Fatal().Msgf(fmt, v...)
}

func (t tailLogger) Panicln(v ...interface{}) {
	t.Logger.Fatal().Msg(fmt.Sprint(v...))
}

func (n *NavMap) watchSlamLog() {
	filePath := filepath.Join(n.flags.RuntimeDirectory, "SLAM_fprintf.log")
	t, err := tail.TailFile(filePath, tail.Config{
		Follow: true,
		Logger: tailLogger{n.logger.With().Str("app", "tail").Logger()},
		ReOpen: true,
	})
	if err != nil {
		n.logger.Fatal().Err(err).Msg("error watching slam log")
	}
	for line := range t.Lines {
		// TODO: needs to clear positions at some point
		l := n.parseSlamLine(line.Text)
		if l.Command == "estimate" {
			n.positionsAppend(v1alpha1.Position{
				X: float32(l.X*20) + 512,
				Y: float32(l.Y*20) + 512,
			})
		}
		if l.Command == "reset" {
			n.positionsEmpty()
		}
	}
}

func (n *NavMap) parseSlamLine(in string) *slamLine {
	content := strings.TrimRight(in, "\n")
	parts := strings.Split(content, " ")
	if len(parts) < 2 {
		return nil
	}

	l := &slamLine{}

	if timestamp, err := strconv.ParseFloat(parts[0], 64); err != nil {
		n.logger.Warn().Err(err).Msgf("error parsing float: %s", err)
	} else {
		l.Timestamp = timestamp
	}

	l.Command = parts[1]

	if len(parts) >= 5 && (l.Command == "estimate" || l.Command == "set_pose") {
		if x, err := strconv.ParseFloat(parts[2], 64); err != nil {
			n.logger.Warn().Err(err).Msgf("error parsing float: %s", err)
			return nil
		} else {
			l.X = x
		}
		if y, err := strconv.ParseFloat(parts[3], 64); err != nil {
			n.logger.Warn().Err(err).Msgf("error parsing float: %s", err)
			return nil
		} else {
			l.Y = y
		}
		if angle, err := strconv.ParseFloat(parts[4], 64); err != nil {
			n.logger.Warn().Err(err).Msgf("error parsing float: %s", err)
			return nil
		} else {
			l.Angle = angle
		}
	}

	if len(parts) >= 3 && (l.Command == "load") {
		if i, err := strconv.ParseInt(parts[2], 10, 64); err != nil {
			n.logger.Warn().Err(err).Msgf("error parsing float: %s", err)
			return nil
		} else {
			l.Int = int(i)
		}
	}

	return l
}
