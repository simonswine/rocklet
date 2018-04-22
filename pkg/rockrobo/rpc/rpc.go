package rpc

import (
	"net/rpc"

	"github.com/rs/zerolog"
)

const SocketPath = "/tmp/rockrobo-rpc.sock"
const Name = "Rockrobo"

type RPC struct {
	r   Rockrobo
	log zerolog.Logger

	SocketPath string
}

type Rockrobo interface {
	LocalAppRCMove(float64, float64, int) error
	LocalAppRCStart() error
	LocalAppRCEnd() error
	LocalAppStart() error
	LocalAppStop() error
	LocalAppPause() error
	LocalAppSpot() error
	LocalAppCharge() error
}

func New(r Rockrobo, log zerolog.Logger) *RPC {
	return &RPC{
		r:          r,
		log:        log.With().Str("server", "rpc").Logger(),
		SocketPath: SocketPath,
	}
}

func NewServer(r *RPC) *rpc.Server {
	s := rpc.NewServer()
	s.RegisterName(Name, r)
	return s
}

type AppRCMoveParams struct {
	Velocity float64
	Omega    float64
	Duration int
}

type Result struct {
	Err error
}

type Params struct {
}

func (r *RPC) AppRCMove(params AppRCMoveParams, result *Result) error {
	r.log.Debug().Msg("received AppRCMove")
	result.Err = r.r.LocalAppRCMove(
		params.Velocity,
		params.Omega,
		params.Duration,
	)
	return nil
}

func (r *RPC) AppRCStart(params Params, result *Result) error {
	r.log.Debug().Msg("received AppRCStart")
	result.Err = r.r.LocalAppRCStart()

	return nil
}

func (r *RPC) AppRCEnd(params Params, result *Result) error {
	r.log.Debug().Msg("received AppRCEnd")
	result.Err = r.r.LocalAppRCEnd()
	return nil
}

func (r *RPC) AppStart(params Params, result *Result) error {
	r.log.Debug().Msg("received AppStart")
	result.Err = r.r.LocalAppStart()
	return nil
}

func (r *RPC) AppStop(params Params, result *Result) error {
	r.log.Debug().Msg("received AppStop")
	result.Err = r.r.LocalAppStop()
	return nil
}

func (r *RPC) AppPause(params Params, result *Result) error {
	r.log.Debug().Msg("received AppPause")
	result.Err = r.r.LocalAppPause()
	return nil
}

func (r *RPC) AppSpot(params Params, result *Result) error {
	r.log.Debug().Msg("received AppSpot")
	result.Err = r.r.LocalAppSpot()
	return nil
}

func (r *RPC) AppCharge(params Params, result *Result) error {
	r.log.Debug().Msg("received AppCharge")
	result.Err = r.r.LocalAppCharge()
	return nil
}
