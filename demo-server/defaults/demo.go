package defaults

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/bintoca/dbuf-demo-go/basic"
	"github.com/bintoca/dbuf-demo-go/connection"
	"github.com/bintoca/dbuf-demo-go/data"
	dbp "github.com/bintoca/dbuf-demo-go/protocol"
	"github.com/joho/godotenv"
	"github.com/quic-go/quic-go"
)

type Settings struct {
	ServerHostName string
	CertPath       string
	PrivKeyPath    string
	ListenAddr     string
	StoragePath    string
	Context        context.Context
	Config         connection.ConnectionConfig
}

func DemoServer(settings Settings) {
	cert, err := tls.LoadX509KeyPair(settings.CertPath, settings.PrivKeyPath)
	if err != nil {
		panic(err)
	}
	tls := &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{dbp.ALPN}}
	addr, err := net.ResolveUDPAddr("udp", settings.ListenAddr)
	if err != nil {
		panic(err)
	}
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	tr := &quic.Transport{
		Conn: udpConn,
	}
	quicConfig := &quic.Config{
		EnableDatagrams: true,
	}
	connection.Listen(settings.Context, tr, tls, quicConfig, settings.Config)
}
func DefaultSettings() Settings {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, relying on system env")
	}
	settings := Settings{
		ServerHostName: os.Getenv("ServerHostName"),
		CertPath:       os.Getenv("CertPath"),
		PrivKeyPath:    os.Getenv("PrivKeyPath"),
		ListenAddr:     os.Getenv("ListenAddr"),
		StoragePath:    os.Getenv("StoragePath"),
		Context:        context.Background(),
		Config:         connection.ConnectionConfig{},
	}
	return settings
}
func DefaultRouteFunc(storagePath string, serverHostName string) (func(rr *dbp.RequestResponse, ctx context.Context) error, data.BadgerStorage) {
	storage, err := data.NewBadgerStorage(storagePath, []basic.DbufVal{basic.Registry(dbp.Registry_host), basic.Text(serverHostName)})
	if err != nil {
		panic(err)
	}
	storage.SetLogger = func(key []byte, value []byte) {
		slog.Info("storage set logger", "key", fmt.Sprint(key), "value", fmt.Sprint(value))
	}
	receiveMessage := func(message basic.DbufVal, authenticatedIdentity basic.DbufValString) error {
		slog.Info("message received: ", "message", message.ToDbufSeq().DebugString(), "identity", authenticatedIdentity.ToDbufSeq().DebugString())
		return nil
	}
	routeFunc := func(rr *dbp.RequestResponse, ctx context.Context) error {
		dbp.RoutePaths(rr, receiveMessage, ctx)
		return nil
	}
	return routeFunc, storage
}