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
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
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
	pixelWall   = 1
	pixelInside = 255
)

func (n *NavMap) ListCleanings(databasePath string) (cleanings []*Cleaning, err error) {
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return []*Cleaning{}, err
	}
	defer db.Close()

	records, err := db.Query("SELECT begin, daybegin, end, code, duration, area, error, complete from cleanrecords;", nil)
	if err != nil {
		return []*Cleaning{}, err
	}
	defer records.Close()

	for records.Next() {
		var begin, daybegin, end int64
		var code, duration, area, errcode, complete int
		err = records.Scan(&begin, &daybegin, &end, &code, &duration, &area, &errcode, &complete)
		if err != nil {
			return nil, err
		}

		cleaning := &Cleaning{
			BeginTime:    time.Unix(begin, 0),
			EndTime:      time.Unix(end, 0),
			DayBeginTime: time.Unix(daybegin, 0),
			Code:         code,
			ErrorCode:    errcode,
			Area:         area,
			Duration:     time.Duration(duration),
			Complete:     complete == 1,
		}

		cleanings = append(cleanings, cleaning)
	}

	maps, err := db.Query("SELECT begin, daybegin, map from cleanmaps;", nil)
	if err != nil {
		return []*Cleaning{}, err
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
			return []*Cleaning{}, err
		}

		if err := n.convertMap(reader); err != nil {
			return []*Cleaning{}, err
		}
	}

	return cleanings, nil

}

func (n *NavMap) parsePath(r io.Reader) error {
	var header struct {
		PointLength uint32
		PointSize   uint32
		Angle       uint32
		ImageWidth1 uint16
		ImageWidth2 uint16
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return err
	}
	n.logger.Debug().Interface("header", &header).Msg("read path header")

	var position struct {
		X uint16
		Y uint16
	}

	for i := 1; i < int(header.PointLength); i++ {
		if err := binary.Read(r, binary.LittleEndian, &position); err != nil {
			return err
		}
	}

	return nil
}

func (n *NavMap) drawMap(r io.Reader, out io.Writer) error {
	var header struct {
		Top    uint32
		Left   uint32
		Height uint32
		Width  uint32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return err
	}

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

	if err := png.Encode(out, i); err != nil {
		return fmt.Errorf("unable to encode image to png: %s", err)
	}

	return nil

}

func (n *NavMap) convertMap(r io.Reader) error {

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

	for {
		var block struct {
			Type   uint16
			Header [2]byte
			Size   uint32
		}
		if err := binary.Read(r, binary.LittleEndian, &block); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		n.logger.Debug().Interface("block_header", &block).Msg("read block header")

		if block.Type == 1 {
			// charger pos
			var chargerPos struct {
				X uint32
				Y uint32
			}
			if err := binary.Read(r, binary.LittleEndian, &chargerPos); err != nil {
				return err
			}
			n.logger.Debug().Msgf("%+v", chargerPos)
		} else if block.Type == 2 {
			f, err := os.Create("/tmp/image.png")
			if err != nil {
				return err
			}
			defer f.Close()
			if err := n.drawMap(r, f); err != nil {
				return err
			}
		} else if block.Type == 3 {
			if err := n.parsePath(r); err != nil {
				return err
			}
		} else {
			buf := new(bytes.Buffer)
			io.CopyN(buf, r, int64(block.Size))
			n.logger.Debug().Str("data", fmt.Sprintf("%#+v", buf.Bytes())).Msgf("received unknown block_type %d with length %d", block.Type, block.Size)
		}
	}

	return nil
}
