package cmd

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/gizak/termui"
)

var logger zerolog.Logger

const middle = 50

type steeringAxis struct {
	value      int
	lock       sync.Mutex
	max        int
	step       int
	stepReduce int
}

func (s *steeringAxis) Add() {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.value < 0 {
		s.value = 0
		return
	}

	s.value += s.step
	if s.value > s.max {
		s.value = s.max
	}
}

func (s *steeringAxis) Sub() {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.value > 0 {
		s.value = 0
		return
	}

	s.value -= s.step
	if s.value < -s.max {
		s.value = -s.max
	}
}

func (s *steeringAxis) GraphValue() int {
	return (s.value + 100) / 2
}

func (s *steeringAxis) Reduce() *steeringAxis {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.value > 0 {
		s.value -= s.stepReduce
	} else if s.value < 0 {
		s.value += s.stepReduce
	}
	return s
}

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "UI test",
	Run: func(cmd *cobra.Command, args []string) {
		err := termui.Init()
		if err != nil {
			panic(err)
		}
		defer termui.Close()

		speedValue := &steeringAxis{
			max:        100,
			step:       25,
			stepReduce: 1,
		}
		speed := termui.NewGauge()
		speed.Percent = 50
		speed.Width = 100
		speed.Height = 3
		speed.PercentColor = termui.ColorBlack
		speed.BarColor = termui.ColorRed
		speed.BorderLabel = "Speed"
		speed.BorderFg = termui.ColorBlack
		speed.BorderLabelFg = termui.ColorBlack

		directionValue := &steeringAxis{
			max:        100,
			step:       25,
			stepReduce: 1,
		}
		direction := termui.NewGauge()
		direction.Percent = 50
		direction.Width = 100
		direction.Height = 3
		direction.PercentColor = termui.ColorBlack
		direction.Y = 3
		direction.BarColor = termui.ColorYellow
		direction.BorderLabel = "left - right"
		direction.BorderFg = termui.ColorBlack
		direction.BorderLabelFg = termui.ColorBlack

		termui.Handle("/sys/kbd/q", func(termui.Event) {
			termui.StopLoop()
		})

		termui.Handle("/sys/kbd/<left>", func(e termui.Event) {
			directionValue.Sub()
		})

		termui.Handle("/sys/kbd/<right>", func(e termui.Event) {
			directionValue.Add()
		})

		termui.Handle("/sys/kbd/<down>", func(e termui.Event) {
			speedValue.Sub()
		})

		termui.Handle("/sys/kbd/<up>", func(e termui.Event) {
			speedValue.Add()
		})

		ticker := time.NewTicker(time.Millisecond * 50)
		go func() {
			for range ticker.C {
				direction.Percent = directionValue.Reduce().GraphValue()
				speed.Percent = speedValue.Reduce().GraphValue()
				termui.Render(direction, speed)
			}
		}()

		termui.Loop()
		ticker.Stop()

	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
