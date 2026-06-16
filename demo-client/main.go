package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/bintoca/dbuf-demo-go/basic"
	"github.com/bintoca/dbuf-demo-go/connection"
	dbp "github.com/bintoca/dbuf-demo-go/protocol"
	"github.com/joho/godotenv"
	"github.com/quic-go/quic-go"
)

type Settings struct {
	ServerName              string
	CustomCertAuthorityPath string
	KeyManagerPath          string
	Context                 context.Context
}

func DefaultSettings() Settings {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found")
	}
	settings := Settings{
		ServerName:              os.Getenv("ServerName"),
		CustomCertAuthorityPath: os.Getenv("CustomCertAuthorityPath"),
		KeyManagerPath:          os.Getenv("KeyManagerPath"),
		Context:                 context.Background(),
	}
	if len(settings.ServerName) == 0 {
		settings.ServerName = "demo.bintoca.net"
	}
	if len(settings.KeyManagerPath) == 0 {
		settings.KeyManagerPath = "./key_manager.json"
	}
	return settings
}
func DefaultTransport() *quic.Transport {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		panic(err)
	}
	tr := &quic.Transport{
		Conn: udpConn,
	}
	return tr
}
func DefaultTLS(settings Settings) *tls.Config {
	tls := &tls.Config{NextProtos: []string{dbp.ALPN}, ServerName: settings.ServerName}
	if len(settings.CustomCertAuthorityPath) > 0 {
		caCert, err := os.ReadFile(settings.CustomCertAuthorityPath)
		if err != nil {
			panic(err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tls.RootCAs = caCertPool
	}
	return tls
}
func DefaultConfig() connection.ConnectionConfig {
	return connection.ConnectionConfig{}
}
func DefaultConnection(settings Settings, tr *quic.Transport, tls *tls.Config, config connection.ConnectionConfig) *connection.DbufConnection {
	addr, err := net.ResolveUDPAddr("udp", settings.ServerName+":443")
	if err != nil {
		panic(err)
	}
	qc := &quic.Config{
		EnableDatagrams: true,
	}
	conn, err := connection.Dial(settings.Context, tr, addr, tls, qc, config)
	if err != nil {
		panic(err)
	}
	return conn
}
func MakeAuthorityRef(dns string) basic.DbufValSeq {
	return basic.DbufValSeq{Val: basic.Array(), Sequence: []basic.DbufValSeq{
		basic.Registry(dbp.Registry_host).ToDbufSeq(),
		basic.Text(dns).ToDbufSeq(),
		basic.Registry(dbp.Registry_authority_marker).ToDbufSeq(),
	}}
}
func LoadKeyManager(settings Settings) *connection.TestKeyManager {
	km := connection.MakeTestKeyManager()
	data, err := os.ReadFile(settings.KeyManagerPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Printf("Error reading file: %v\n", err)
		}
	} else {
		slog.Info("Loading key manager state", "path", settings.KeyManagerPath)
		var keys []connection.KeyInit
		err := json.Unmarshal(data, &keys)
		if err != nil {
			panic(err)
		}
		for _, key := range keys {
			b, err := basic.EncodeFullSeq(key.AuthorityRef)
			if err != nil {
				panic(err)
			}
			authorityRefString := string(b)
			km.Keys[authorityRefString] = key
		}
	}
	km.DataChangeFunc = func(km connection.TestKeyManager) {
		keys := make([]connection.KeyInit, 0, len(km.Keys))
		for _, value := range km.Keys {
			keys = append(keys, value)
		}
		jsonData, err := json.Marshal(keys)
		if err != nil {
			panic(err)
		}
		slog.Info("Saving key manager state", "path", settings.KeyManagerPath)
		err = os.WriteFile(settings.KeyManagerPath, jsonData, 0600)
		if err != nil {
			panic(err)
		}
	}
	return km
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	settings := DefaultSettings()
	tr := DefaultTransport()
	tls := DefaultTLS(settings)
	config := DefaultConfig()
	km := LoadKeyManager(settings)
	fmt.Print("Enter a message to send to " + settings.ServerName + " with the DBUF protocol: ")

	if scanner.Scan() {
		input := scanner.Text()
		conn := DefaultConnection(settings, tr, tls, config)
		tps := dbp.MakeHeaderStoreState(km, conn)
		message := basic.Text(input).ToDbufSeq()
		slog.Info("Sending message")
		err := dbp.SendMessage(tps, MakeAuthorityRef(settings.ServerName), message, settings.Context)
		if err != nil {
			slog.Error("Send message failed:", slog.Any("err", err))
		} else {
			slog.Info("Send message succeeded")
		}
	}
}
