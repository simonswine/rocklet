package navmap

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/binary"
	"fmt"
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

		out, err := os.Create(fmt.Sprintf("map_%d.ppm", begin))
		if err != nil {
			return []*Cleaning{}, err
		}
		defer out.Close()

		_, err = io.Copy(out, reader)
		if err != nil {
			return []*Cleaning{}, err
		}

	}

	return cleanings, nil

}

func (n *NavMap) convertMap(r io.Reader) error {

	var header struct {
		Magic           [2]byte
		HeaderLen       int16
		ChecksumPointer [4]byte
		MajorVer        int16
		MinorVer        int16
		MapIndex        int32
		MapSequence     int32
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return err
	}
	n.logger.Debug().Interface("header", &header).Msg("read map header")

	var chargerPos struct {
		X int32
		Y int32
	}

	for {
		var block struct {
			Type    int16
			Unknown [2]byte
			Size    int32
		}
		if err := binary.Read(r, binary.LittleEndian, &block); err != nil {
			return err
		}
		n.logger.Debug().Interface("block_header", &block).Msg("read block header")

		buf := new(bytes.Buffer)
		io.CopyN(buf, r, int64(block.Size))

		if block.Type == 1 {
			if err := binary.Read(r, binary.LittleEndian, &block); err != nil {
				return err
			}
			// charger pos
			if err := binary.Read(r, binary.LittleEndian, &chargerPos); err != nil {
				return err
			}
		} else if block.Type == 2 {
			// map
		} else if block.Type == 3 {
			// path
		} else if block.Type < -1 || block.Type > 8 {
			break
		} else if block.Size > 0 {
			n.logger.Debug().Msgf("unhandled block_type %d with length %d", block.Type, block.Size)
		}
	}
	n.logger.Debug().Msgf("%+v", chargerPos)

	return nil
}
