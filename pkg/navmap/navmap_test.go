package navmap

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"
)

var exampleSLAMMap = `12.361 pause
12.613 load 1
12.734 set_pose 0.104 -0.017 -3.122
25601.569 pause
25601.837 pause
25602.126 resume
25602.336 reset
25608.800 unlock
25612.945 lock
25612.945 reset
25614.681 estimate 0.000 -0.002 -1.637
25619.296 estimate 0.008 0.049 1.162
`

func TestMap_New(t *testing.T) {
	m, err := NewMap("navmap0.ppm", time.Now())

	if err != nil {
		t.Fatal("failed to create map: ", err)
	}

	data, err := m.PNG()
	if err != nil {
		t.Fatal("failed to create png: ", err)
	}

	err = ioutil.WriteFile("test1.png", data, 0644)
	if err != nil {
		t.Fatal("failed to create png: ", err)
	}
}

func TestSlamMap(t *testing.T) {

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}

	n := newNavMap()
	n.flags.RuntimeDirectory = tempDir

	done := make(chan struct{}, 0)

	go func() {
		n.watchSlamLog()
		close(done)

	}()

	if err := ioutil.WriteFile(fmt.Sprintf("%s/%s", tempDir, "SLAM_fprintf.log"), []byte(exampleSLAMMap), 0644); err != nil {
		t.Fatal("unexpected error: ", err)
	}

	for {
		time.Sleep(time.Nanosecond)
		p := n.Positions()
		n.logger.Info().Msgf("positions %+v", p)
		if len(p) == 2 {
			break
		}
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/%s", tempDir, "SLAM_fprintf.log"), []byte(`25612.945 reset
25614.681 estimate 0 0 1.637
`), 0644); err != nil {
		t.Fatal("unexpected error: ", err)
	}

	for {
		time.Sleep(time.Nanosecond)
		p := n.Positions()
		n.logger.Info().Msgf("positions %+v", p)
		if len(p) == 1 {
			break
		}
	}

}

func TestSlamLineParse(t *testing.T) {
	n := newNavMap()

	l := n.parseSlamLine("12.613 load 1")
	// check for load
	if exp, act := 1, l.Int; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}
	if exp, act := "load", l.Command; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}

	// check for set_pose
	l = n.parseSlamLine("12.734 set_pose 0.104 -0.017 -3.122")
	if exp, act := 0.104, l.X; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}
	if exp, act := -0.017, l.Y; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}
	if exp, act := "set_pose", l.Command; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}

	// check for estimate lines
	l = n.parseSlamLine("25619.296 estimate 0.008 0.049 1.162")
	if exp, act := 0.008, l.X; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}
	if exp, act := 0.049, l.Y; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}
	if exp, act := "estimate", l.Command; exp != act {
		t.Errorf("unexpected result exp=%v, act=%v", exp, act)
	}
}
