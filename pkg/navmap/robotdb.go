package navmap

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/simonswine/rocklet/pkg/apis/vacuum/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Cleaning struct {
	BeginTime    time.Time
	EndTime      time.Time
	DayBeginTime time.Time
	Duration     time.Duration
	Area         int
	ErrorCode    int
	Code         int
	Complete     bool
}

const (
	pixelWall    = 1
	pixelInside  = 255
	pixelUnknown = 0
)

func (n *NavMap) ListCleanings() (cleanings []*v1alpha1.Cleaning, err error) {
	db, err := sql.Open("sqlite3", n.flags.RobotDatabase)
	if err != nil {
		return []*v1alpha1.Cleaning{}, err
	}
	defer db.Close()

	records, err := db.Query("SELECT begin, daybegin, end, code, duration, area, error, complete from cleanrecords;", nil)
	if err != nil {
		return []*v1alpha1.Cleaning{}, err
	}
	defer records.Close()

	indexByBegin := make(map[int64]*v1alpha1.Cleaning)

	for records.Next() {
		var begin, daybegin, end int64
		var code, duration, area, errcode, complete int
		err = records.Scan(&begin, &daybegin, &end, &code, &duration, &area, &errcode, &complete)
		if err != nil {
			return nil, err
		}

		completeValue := complete == 1

		cleaning := &v1alpha1.Cleaning{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%d", n.flags.Kubernetes.NodeName, begin),
			},
			Spec: v1alpha1.CleaningSpec{
				NodeName: n.flags.Kubernetes.NodeName,
			},
			Status: v1alpha1.CleaningStatus{
				BeginTime:    &metav1.Time{Time: time.Unix(begin, 0)},
				EndTime:      &metav1.Time{Time: time.Unix(end, 0)},
				DayBeginTime: &metav1.Time{Time: time.Unix(daybegin, 0)},
				Code:         &code,
				ErrorCode:    &errcode,
				Area:         &area,
				Duration:     (time.Second * time.Duration(duration)).String(),
				Complete:     &completeValue,
			},
		}
		indexByBegin[begin] = cleaning

		cleanings = append(cleanings, cleaning)
	}

	maps, err := db.Query("SELECT begin, daybegin, map from cleanmaps;", nil)
	if err != nil {
		return []*v1alpha1.Cleaning{}, err
	}
	defer maps.Close()

	for maps.Next() {
		var begin, daybegin int64
		var mapData []byte
		err = maps.Scan(&begin, &daybegin, &mapData)
		if err != nil {
			return nil, err
		}

		reader, err := gzip.NewReader(bytes.NewReader(mapData))
		if err != nil {
			return []*v1alpha1.Cleaning{}, err
		}

		cleaning, ok := indexByBegin[begin]
		if !ok {
			n.Logger().Error().Time("begin", time.Unix(begin, 0)).Msg("can't match map to cleaning record")
			continue
		}

		err = n.ConvertMap(reader, cleaning)
		if err != nil {
			return []*v1alpha1.Cleaning{}, err
		}
	}

	return cleanings, nil

}

type position struct {
	X uint16
	Y uint16
}

type chargerPos struct {
	X uint32
	Y uint32
}

func (n *NavMap) parsePath(r io.Reader) ([]position, error) {
	var header struct {
		PointLength uint32
		PointSize   uint32
		Angle       uint32
		ImageWidth  uint16
		ImageHeight uint16
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return []position{}, err
	}
	n.logger.Debug().Interface("header", &header).Msg("read path header")

	var path []position
	var pos position

	for i := 1; i < int(header.PointLength); i++ {
		if err := binary.Read(r, binary.LittleEndian, &pos); err != nil {
			return []position{}, err
		}
		path = append(path, pos)
	}

	return path, nil
}

func (n *NavMap) drawMap(r io.Reader) (*v1alpha1.Map, error) {
	var header struct {
		Top    uint32
		Left   uint32
		Height uint32
		Width  uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return nil, err
	}
	n.logger.Debug().Interface("header", &header).Msg("read image header")

	buf := new(bytes.Buffer)
	io.CopyN(buf, r, int64(header.Height*header.Width))

	i := image.NewRGBA(image.Rectangle{
		Min: image.Pt(0, 0),
		Max: image.Pt(int(header.Width), int(header.Height)),
	})

	colorConvert := func(b byte) color.RGBA {
		if b == pixelWall {
			return color.RGBA{
				R: 105,
				G: 207,
				B: 254,
				A: 255,
			}
		} else if b == pixelInside {
			return color.RGBA{
				R: 33,
				G: 115,
				B: 187,
				A: 255,
			}
		} else if b == pixelUnknown {
			return color.RGBA{}
		}
		return color.RGBA{
			R: b,
			G: b,
			B: b,
			A: 255,
		}
	}

	// write byte by byte
	var width = int(header.Width)
	var height = int(header.Height)

	for pos, b := range buf.Bytes() {
		i.SetRGBA((pos % width), height-((pos/width)+1), colorConvert(b))
	}

	buf = new(bytes.Buffer)
	if err := png.Encode(buf, i); err != nil {
		return nil, fmt.Errorf("unable to encode image to png: %s", err)
	}

	return &v1alpha1.Map{
		Top:    header.Top,
		Left:   header.Left,
		Height: header.Height,
		Width:  header.Width,
		Data:   buf.Bytes(),
	}, nil

}

func (n *NavMap) ConvertMap(r io.Reader, cleaning *v1alpha1.Cleaning) error {

	var header struct {
		Magic           [2]byte
		HeaderLen       uint16
		ChecksumPointer [4]byte
		MajorVer        uint16
		MinorVer        uint16
		MapIndex        uint32
		MapSequence     uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return err
	}
	n.logger.Debug().Interface("header", &header).Msg("read map header")

	var charger *chargerPos
	var path *[]position

	for {
		var block struct {
			Type   uint16
			Header [2]byte
			Size   uint32
		}
		if err := binary.Read(r, binary.LittleEndian, &block); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		n.logger.Debug().Interface("block_header", &block).Msg("read block header")

		if block.Type == 1 {
			// charger pos
			var ch chargerPos
			if err := binary.Read(r, binary.LittleEndian, &ch); err != nil {
				return err
			}
			n.logger.Debug().Msgf("%+v", ch)
			charger = &ch
		} else if block.Type == 2 {
			m, err := n.drawMap(r)
			if err != nil {
				return err
			}
			cleaning.Status.Map = m
		} else if block.Type == 3 {
			p, err := n.parsePath(r)
			if err != nil {
				return err
			}
			path = &p
		} else {
			buf := new(bytes.Buffer)
			io.CopyN(buf, r, int64(block.Size))
			n.logger.Debug().Str("data", fmt.Sprintf("%#+v", buf.Bytes())).Msgf("received unknown block_type %d with length %d", block.Type, block.Size)
		}
	}

	if cleaning.Status.Map == nil {
		return fmt.Errorf("no map found")
	}

	if path != nil {
		p := *path
		cleaning.Status.Path = make([]v1alpha1.Position, len(p))
		for i, _ := range p {
			cleaning.Status.Path[i] = positionInMapDimensions(cleaning.Status.Map, uint32(p[i].X), uint32(p[i].Y))
		}
	}

	if charger != nil {
		cleaning.Status.Charger = positionInMapDimensions(cleaning.Status.Map, charger.X, charger.Y)
	}

	return nil
}

// converts coordinates into pngs dimensions
func positionInMapDimensions(m *v1alpha1.Map, X uint32, Y uint32) v1alpha1.Position {
	return v1alpha1.Position{
		X: (float32(X) * 0.02) - float32(m.Left),
		Y: float32(m.Height) - (float32(Y) * 0.02) + float32(m.Top),
	}
}
