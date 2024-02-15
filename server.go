package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Option func(option *options) error

type options struct {
	host           *string
	port           *string
	maxheaderbytes *int
	writetimeout   *time.Duration
	readtimeout    *time.Duration
	idletimeout    *time.Duration
}

const (
	default_write_timeout = time.Duration(15 * time.Second)
	default_read_timeout  = time.Duration(15 * time.Second)
	default_idle_timeout  = time.Duration(60 * time.Second)
)

type Server struct {
	*http.Server
}

func New(ctx context.Context, handler http.Handler, opts ...Option) (*Server, error) {
	if handler == nil {
		return nil, fmt.Errorf("undefined handler")
	}
	var opt options
	for _, option := range opts {
		if err := option(&opt); err != nil {
			return nil, err
		}
	}

	host := ""
	if opt.host != nil {
		host = *opt.host
	}
	port := ""
	if opt.port != nil {
		port = *opt.port
	}
	_, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return nil, err
	}
	var writetimeout, readtimeout, idletimeout time.Duration
	if opt.writetimeout == nil {
		writetimeout = default_write_timeout
	} else {
		writetimeout = *opt.writetimeout
	}
	if opt.readtimeout == nil {
		readtimeout = default_read_timeout
	} else {
		readtimeout = *opt.readtimeout
	}
	if opt.idletimeout == nil {
		idletimeout = default_idle_timeout
	} else {
		idletimeout = *opt.idletimeout
	}
	var maxheaderbytes int
	if opt.maxheaderbytes == nil {
		maxheaderbytes = http.DefaultMaxHeaderBytes
	} else {
		maxheaderbytes = *opt.maxheaderbytes
	}
	sctx, cancel := context.WithCancel(ctx)
	s := &http.Server{
		Addr:           fmt.Sprintf("%s:%s", host, port),
		Handler:        handler,
		WriteTimeout:   writetimeout,
		ReadTimeout:    readtimeout,
		IdleTimeout:    idletimeout,
		MaxHeaderBytes: maxheaderbytes,
		BaseContext:    func(_ net.Listener) context.Context { return sctx },
	}
	s.RegisterOnShutdown(cancel)
	return &Server{s}, nil
}

func WithMaxHeaderBytes(bts int) Option {
	return func(options *options) error {
		options.maxheaderbytes = &bts
		return nil
	}
}

func WithWriteTimeout(timeout time.Duration) Option {
	return func(options *options) error {
		options.writetimeout = &timeout
		return nil
	}
}

func WithReadTimeout(timeout time.Duration) Option {
	return func(options *options) error {
		options.readtimeout = &timeout
		return nil
	}
}

func WithIdleTimeout(timeout time.Duration) Option {
	return func(options *options) error {
		options.idletimeout = &timeout
		return nil
	}
}

func WithHost(host string) Option {
	return func(options *options) error {
		options.host = &host
		return nil
	}
}

// if port=0 listening to random available port
func WithPort(port int) Option {
	return func(options *options) error {
		if port < 0 {
			return fmt.Errorf("port cannot be less than zero")
		}
		p := fmt.Sprintf("%d", port)
		options.port = &p
		return nil
	}
}

func (s *Server) StartWithAwaitStop(stoptimeout time.Duration) error {
	go func() {
		 s.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig,
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGABRT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
		syscall.SIGHUP,
	)
	<-sig

	gracefullCtx, cancelShutdown := context.WithTimeout(s.BaseContext(nil), stoptimeout)
	defer cancelShutdown()
	s.SetKeepAlivesEnabled(false)
	
	return s.Shutdown(gracefullCtx)
}


