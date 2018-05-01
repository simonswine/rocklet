package cmd

import (
	"fmt"
	netrpc "net/rpc"
	"os"
	"sync"
	"time"

	"github.com/gizak/termui"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/simonswine/rocklet/pkg/rockrobo/rpc"
)

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
	RunE: func(cmd *cobra.Command, args []string) error {

		remoteControl := false
		remoteControlLock := new(sync.Mutex)

		logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel)

		tries := 0
		var client *netrpc.Client
		for {
			var err error
			logger.Info().Str("socket_path", rpc.SocketPath).Msg("trying to connect to the RPC server")
			client, err = netrpc.Dial("unix", rpc.SocketPath)
			if err == nil {
				break
			}

			logger.Error().Err(err).Msg("error connecting to RPC server")

			if tries >= 10 {
				return fmt.Errorf("failed to connect to RPC server after %d tries", tries)
			}
			tries++
			time.Sleep(100 * time.Millisecond)
		}
		defer client.Close()

		// on exit disable remote control
		defer func() {
			if remoteControl {
				var result rpc.Result
				if err := client.Call(fmt.Sprintf("%s.AppRCEnd", rpc.Name), &rpc.Params{}, &result); err != nil {
					logger.Error().Err(err).Msgf("failed to stop remote control")
				} else if result.Err != nil {
					logger.Error().Err(result.Err).Msgf("failed to stop remote control")
				}
			}
		}()

		err := termui.Init()
		if err != nil {
			return err
		}
		defer termui.Close()

		speedValue := &steeringAxis{
			min:   -0.29,
			max:   0.29,
			start: 0.0,
			step:  0.05,
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
			min:   -3.141,
			max:   3.141,
			start: 0.0,
			step:  0.1,
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

		rpcProxy := func(cmd string) func() {
			return func() {
				var result rpc.Result
				if err := client.Call(fmt.Sprintf("%s.%s", rpc.Name, cmd), &rpc.Params{}, &result); err != nil {
					logger.Fatal().Err(err).Msgf("failed to %s", cmd)
				} else if result.Err != nil {
					logger.Fatal().Err(result.Err).Msgf("failed to %s", cmd)
				}
				logger.Info().Str("command", cmd).Msg("rpc sent")
			}
		}

		rpcRCStart := func() {
			remoteControlLock.Lock()
			defer remoteControlLock.Unlock()
			rpcProxy("AppRCStart")()
			remoteControl = true
		}

		rpcRCEnd := func() {
			remoteControlLock.Lock()
			defer remoteControlLock.Unlock()
			rpcProxy("AppRCEnd")()
			remoteControl = false
		}

		// reset  movements
		termui.Handle("/sys/kbd/s", func(termui.Event) {
			directionValue.Set(directionValue.start)
			speedValue.Set(speedValue.start)
		})
		commands := []struct {
			n string
			f func()
		}{
			{"a", rpcRCStart},
			{"d", rpcRCEnd},
			{"x", rpcProxy("AppSpot")},
			{"c", rpcProxy("AppCharge")},
			{"v", rpcProxy("AppStart")},
			{"b", rpcProxy("AppPause")},
			{"n", rpcProxy("AppStop")},
		}

		for i, _ := range commands {
			f := commands[i].f
			termui.Handle(fmt.Sprintf("/sys/kbd/%s", commands[i].n), func(termui.Event) {
				f()
			})
		}

		termui.Handle("/sys/kbd/<left>", func(e termui.Event) {
			directionValue.Add()
		})

		termui.Handle("/sys/kbd/<right>", func(e termui.Event) {
			directionValue.Sub()
		})

		termui.Handle("/sys/kbd/<down>", func(e termui.Event) {
			speedValue.Sub()
		})

		termui.Handle("/sys/kbd/<up>", func(e termui.Event) {
			speedValue.Add()
		})

		tickerUI := time.NewTicker(time.Millisecond * 50)
		go func() {
			for range tickerUI.C {
				direction.Percent = directionValue.Reduce().GraphValue()
				speed.Percent = speedValue.Reduce().GraphValue()
				termui.Render(direction, speed)
			}
		}()

		tickerRC := time.NewTicker(time.Millisecond * 250)
		go func() {
			for range tickerRC.C {
				if remoteControl {
					var result rpc.Result
					if err := client.Call(fmt.Sprintf("%s.AppRCMove", rpc.Name), &rpc.AppRCMoveParams{
						Duration: 500,
						Omega:    directionValue.value,
						Velocity: speedValue.value,
					}, &result); err != nil {
						logger.Error().Err(err).Msgf("failed to send remote control move")
					} else if result.Err != nil {
						logger.Error().Err(result.Err).Msgf("failed to send remote control move")
					}
				}
			}
		}()

		termui.Loop()
		tickerRC.Stop()
		tickerUI.Stop()
		return nil

	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
