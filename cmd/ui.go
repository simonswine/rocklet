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
	value      float64
	lock       sync.Mutex
	max        float64
	min        float64
	start      float64
	step       float64
	stepReduce int
}

func (s *steeringAxis) Set(value float64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.value = value
}

func (s *steeringAxis) Add() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.value += s.step
	if s.value > s.max {
		s.value = s.max
	}
}

func (s *steeringAxis) Sub() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.value -= s.step
	if s.value < s.min {
		s.value = s.min
	}
}

func (s *steeringAxis) GraphValue() int {
	return int(((s.value - s.min) / (s.max - s.min)) * 100)
}

func (s *steeringAxis) Reduce() *steeringAxis {
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
			min:   -3.0,
			max:   3.0,
			start: 0.0,
			step:  0.06,
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
			min:   -0.2,
			max:   0.2,
			start: 0.0,
			step:  0.004,
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

		termui.Handle("/sys/kbd/s", func(termui.Event) {
			directionValue.Set(directionValue.start)
			speedValue.Set(speedValue.start)
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
