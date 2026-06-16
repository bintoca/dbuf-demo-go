package connection

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/bintoca/dbuf-demo-go/basic"
	"github.com/bintoca/dbuf-demo-go/data"
	dbp "github.com/bintoca/dbuf-demo-go/protocol"
	"github.com/quic-go/quic-go"
)

type ConnectionConfig struct {
	RouteFuncBi       func(rr *dbp.RequestResponse, ctx context.Context) error
	RouteFuncUni      func(rr *dbp.RequestResponse, ctx context.Context) error
	RouteFuncDatagram func(rr *dbp.RequestResponse, ctx context.Context) error
	SetupConnection   func(conn *quic.Conn, tc *dbp.HeaderStoreCache)
	AutoClose         bool
	Storage           data.Storage
}

func HandleConnection(conn *quic.Conn, tc *dbp.HeaderStoreCache, ctx context.Context, connConfig ConnectionConfig) {
	if connConfig.AutoClose {
		defer conn.CloseWithError(quic.ApplicationErrorCode(0), "")
	}
	if connConfig.SetupConnection != nil {
		connConfig.SetupConnection(conn, tc)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		if connConfig.RouteFuncBi != nil {
			LoopStreams(conn, tc, connConfig.RouteFuncBi, ctx)
		}
	}()
	go func() {
		defer wg.Done()
		if connConfig.RouteFuncUni != nil {
			LoopUniStreams(conn, tc, connConfig.RouteFuncUni, ctx)
		}
	}()
	go func() {
		defer wg.Done()
		if connConfig.RouteFuncDatagram != nil {
			LoopDatagram(conn, tc, connConfig.RouteFuncDatagram, ctx)
		}
	}()
	wg.Wait()
}
func LoopStreams(conn *quic.Conn, tc *dbp.HeaderStoreCache, routeFunc func(rr *dbp.RequestResponse, ctx context.Context) error, ctx context.Context) {
	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				slog.Debug("Accept stream timeout", slog.Any("err", err))
				return
			}
			if errors.Is(err, context.Canceled) {
				slog.Debug("Accept stream context canceled", slog.Any("err", err))
				return
			}
			slog.Error("Accept stream error", slog.Any("err", err))
			break
		}
		go HandleStream(stream, conn, tc, routeFunc, ctx)
	}
}
func LoopUniStreams(conn *quic.Conn, tc *dbp.HeaderStoreCache, routeFunc func(rr *dbp.RequestResponse, ctx context.Context) error, ctx context.Context) {
	for {
		stream, err := conn.AcceptUniStream(ctx)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				slog.Debug("Accept uni stream timeout", slog.Any("err", err))
				return
			}
			if errors.Is(err, context.Canceled) {
				slog.Debug("Accept uni stream context canceled", slog.Any("err", err))
				return
			}
			slog.Error("Accept uni stream error", slog.Any("err", err))
			break
		}
		go HandleUniStream(stream, tc, routeFunc, ctx)
	}
}
func LoopDatagram(conn *quic.Conn, tc *dbp.HeaderStoreCache, routeFunc func(rr *dbp.RequestResponse, ctx context.Context) error, ctx context.Context) {
	for {
		b, err := conn.ReceiveDatagram(ctx)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				slog.Debug("Receive datagram timeout", slog.Any("err", err))
				return
			}
			if errors.Is(err, context.Canceled) {
				slog.Debug("Receive datagram context canceled", slog.Any("err", err))
				return
			}
			slog.Error("Receive datagram error", slog.Any("err", err))
			break
		}
		go HandleDatagram(b, conn, tc, routeFunc, ctx)
	}
}
func HandleStream(stream *quic.Stream, conn *quic.Conn, tc *dbp.HeaderStoreCache, routeFunc func(rr *dbp.RequestResponse, ctx context.Context) error, ctx context.Context) {
	defer stream.CancelRead(0)
	streamGroup, executeErr := dbp.Execute(stream, stream, tc, routeFunc, ctx)
	if executeErr != nil {
		stream.CancelWrite(basic.Registry_error)
		slog.Error("Execute failed", slog.Any("err", executeErr))
		es, err := conn.OpenUniStreamSync(ctx)
		if err != nil {
			slog.Error("OpenUniStreamSync failed", slog.Any("err", err))
			return
		}
		defer es.Close()
		if streamGroup > 0 {
			err = dbp.EncodeWrite(es, basic.Uint(streamGroup).ToDbufSeq())
			if err != nil {
				slog.Error("Stream group encode failed: %w", slog.Any("err", err))
				return
			}
		}
		err = dbp.EncodeWrite(es, dbp.MakeErrorOnOtherStream(uint64(stream.StreamID()), basic.ErrorToVal(executeErr)))
		if err != nil {
			slog.Error("Error on alternate stream encode failed: %w, %w", slog.Any("err", err), slog.Any("executeErr", executeErr))
			return
		}
	} else {
		err := stream.Close()
		if err != nil {
			slog.Error("Stream close failed: %w", slog.Any("err", err))
		}
	}
}
func HandleUniStream(stream *quic.ReceiveStream, tc *dbp.HeaderStoreCache, routeFunc func(rr *dbp.RequestResponse, ctx context.Context) error, ctx context.Context) {
	defer stream.CancelRead(0)
	_, err := dbp.Execute(stream, nil, tc, routeFunc, ctx)
	if err != nil {
		slog.Error("Execute uni failed: %w", slog.Any("err", err))
	}
}
func HandleDatagram(b []byte, conn *quic.Conn, tc *dbp.HeaderStoreCache, routeFunc func(rr *dbp.RequestResponse, ctx context.Context) error, ctx context.Context) {
	_, err := dbp.Execute(bytes.NewReader(b), nil, tc, routeFunc, ctx)
	if err != nil {
		slog.Error("Execute datagram failed: %w", slog.Any("err", err))
	}
}
func Listen(ctx context.Context, tr *quic.Transport, tlsConfig *tls.Config, quicConfig *quic.Config, connConfig ConnectionConfig) {
	ln, err := tr.Listen(tlsConfig, quicConfig)
	if err != nil {
		slog.Error("Listen failed: %w", slog.Any("err", err))
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept(ctx)
		if err != nil {
			slog.Error("Accept connection failed: %w", slog.Any("err", err))
			break
		}
		tls := conn.ConnectionState().TLS
		tc := dbp.MakeHeaderStoreCache(connConfig.Storage, tls.ExportKeyingMaterial)
		go HandleConnection(conn, tc, ctx, connConfig)
	}
}
func Dial(ctx context.Context, tr *quic.Transport, addr net.Addr, tlsConfig *tls.Config, quicConfig *quic.Config, connConfig ConnectionConfig) (*DbufConnection, error) {
	conn, err := tr.Dial(ctx, addr, tlsConfig, quicConfig)
	if err != nil {
		return nil, fmt.Errorf("Dial failed: %w", err)
	}
	tls := conn.ConnectionState().TLS
	tc := dbp.MakeHeaderStoreCache(connConfig.Storage, tls.ExportKeyingMaterial)
	go HandleConnection(conn, tc, ctx, connConfig)
	return &DbufConnection{QuicConnection: conn, Cache: tc}, nil
}

type DbufConnection struct {
	QuicConnection *quic.Conn
	Cache          *dbp.HeaderStoreCache
}

func (c *DbufConnection) OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error) {
	return c.QuicConnection.OpenStreamSync(ctx)
}
func (c *DbufConnection) HeaderStoreCache() *dbp.HeaderStoreCache {
	return c.Cache
}
