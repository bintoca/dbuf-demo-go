package connection

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/bintoca/dbuf-demo-go/basic"
	"github.com/bintoca/dbuf-demo-go/data"
	dbp "github.com/bintoca/dbuf-demo-go/protocol"
	"github.com/quic-go/quic-go"
)

func getTlsConfigTest() *tls.Config {
	cert, err := tls.LoadX509KeyPair("./test-certs/ed25519/end.fullchain", "./test-certs/ed25519/end.key")
	if err != nil {
		panic(err)
	}
	caCert, err := os.ReadFile("./test-certs/ed25519/ca.cert")
	if err != nil {
		panic(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{dbp.ALPN}, RootCAs: caCertPool, ServerName: "localhost"}
}
func getQuicConfigTest() *quic.Config {
	return &quic.Config{
		MaxIdleTimeout:  45 * time.Second,
		KeepAlivePeriod: 30 * time.Second,
		EnableDatagrams: true,
	}
}
func getAddrTest() *net.UDPAddr {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8081")
	if err != nil {
		panic(err)
	}
	return addr
}
func getTransportTest() *quic.Transport {
	udpConn, err := net.ListenUDP("udp", getAddrTest())
	if err != nil {
		panic(err)
	}
	tr := &quic.Transport{
		Conn: udpConn,
	}
	return tr
}
func getListenDial(listenConfig ConnectionConfig, dialConfig ConnectionConfig) (*DbufConnection, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	tr := getTransportTest()
	go Listen(ctx, tr, getTlsConfigTest(), getQuicConfigTest(), listenConfig)
	conn, err := Dial(ctx, tr, getAddrTest(), getTlsConfigTest(), getQuicConfigTest(), dialConfig)
	if err != nil {
		panic(err)
	}
	return conn, ctx, cancel
}
func echo(rr *dbp.RequestResponse, _ context.Context) error {
	rr.ResponseHeaders.HasValue = true
	rr.ResponseHeaders.Value = rr.RequestHeaderParams.Header.ToDbufSeq()
	return nil
}
func getListenDialMessage(rc dbp.ReceiveMessageFunc) (*DbufConnection, context.Context, context.CancelFunc) {
	listenStore, _ := data.NewBadgerMemoryStorage([]basic.DbufVal{basic.Registry(dbp.Registry_host), basic.Text("localhost")})
	dialStore, _ := data.NewBadgerMemoryStorage(nil)
	routeFunc := func(rr *dbp.RequestResponse, ctx context.Context) error {
		dbp.RoutePaths(rr, rc, ctx)
		return nil
	}
	return getListenDial(ConnectionConfig{AutoClose: true,
		RouteFuncBi: routeFunc,
		Storage:     listenStore,
	},
		ConnectionConfig{AutoClose: true,
			RouteFuncBi: echo,
			Storage:     dialStore,
		})
}
func localAuthorityRef() basic.DbufValSeq {
	return basic.DbufValSeq{Val: basic.Array(), Sequence: []basic.DbufValSeq{
		basic.Registry(dbp.Registry_host).ToDbufSeq(),
		basic.Text("localhost").ToDbufSeq(),
		basic.Registry(dbp.Registry_authority_marker).ToDbufSeq(),
	}}
}

var RegNames = map[uint32]string{
	basic.Registry_nonexistent:             "nonexistent",
	basic.Registry_describe_no_value:       "describe_no_value",
	basic.Registry_value:                   "value",
	basic.Registry_error:                   "error",
	basic.Registry_error_internal:          "error_internal",
	basic.Registry_incomplete_stream:       "incomplete_stream",
	basic.Registry_data_error:              "data_error",
	basic.Registry_data_type_not_accepted:  "data_type_not_accepted",
	basic.Registry_data_value_not_accepted: "data_value_not_accepted",
	basic.Registry_data_key_not_accepted:   "data_key_not_accepted",
	basic.Registry_data_key_missing:        "data_key_missing",
	basic.Registry_end_marker:              "end_marker",
	basic.Registry_data_path:               "data_path",
	basic.Registry_magic_number:            "magic_number",
	dbp.Registry_authority_marker:          "authority_marker",
	dbp.Registry_host:                      "host",
	dbp.Registry_reference:                 "reference",
	dbp.Registry_header_store:              "transport_store",
	dbp.Registry_operation:                 "operation",
	dbp.Registry_body_length:               "body_length",
	dbp.Registry_not_authenticated:         "not_authenticated",
	dbp.Registry_identity:                  "identity",
	dbp.Registry_identity_key:              "identity_key",
	dbp.Registry_identity_recovery:         "identity_recovery",
	dbp.Registry_deliver_message:           "deliver_message",
	dbp.Registry_ed25519:                   "ed25519",
	dbp.Registry_stream_group:              "stream_group",
	dbp.Registry_header:                    "header",
	dbp.Registry_body:                      "body",
	dbp.Registry_footer:                    "footer",
}

func TestMessage(t *testing.T) {
	basic.DebugRegistryNames = RegNames
	var logs []basic.DbufValString
	conn, ctx, cancel := getListenDialMessage(func(message basic.DbufVal, authenticatedIdentity basic.DbufValString) error {
		logs = append(logs, message.ToDbufValString())
		return nil
	})
	t.Cleanup(cancel)
	tps := dbp.MakeHeaderStoreState(MakeTestKeyManager(), conn)
	message := basic.DbufValSeq{Val: basic.Text("hello world")}
	err := dbp.SendMessage(tps, localAuthorityRef(), message, ctx)
	if err != nil {
		t.Error("SendMessage failed:", err)
	}
	err = dbp.SendMessage(tps, localAuthorityRef(), message, ctx)
	if err != nil {
		t.Error("SendMessage failed:", err)
	}
	expectedLogs := []basic.DbufValString{basic.Text("hello world").ToDbufValString(), basic.Text("hello world").ToDbufValString()}
	if !slices.Equal(logs, expectedLogs) {
		t.Error("Logs not equal:", logs, expectedLogs)
	}
}
