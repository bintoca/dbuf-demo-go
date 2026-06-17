package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"

	"github.com/bintoca/dbuf-demo-go/basic"
	"github.com/bintoca/dbuf-demo-go/demo-client/defaults"
	dbp "github.com/bintoca/dbuf-demo-go/protocol"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	settings := defaults.DefaultSettings()
	tr := defaults.DefaultTransport()
	tls := defaults.DefaultTLS(settings)
	config := defaults.DefaultConfig()
	km := defaults.LoadKeyManager(settings)
	fmt.Print("Enter a message to send to " + settings.ServerName + " with the DBUF protocol: ")

	if scanner.Scan() {
		input := scanner.Text()
		conn := defaults.DefaultConnection(settings, tr, tls, config)
		tps := dbp.MakeHeaderStoreState(km, conn)
		message := basic.Text(input).ToDbufSeq()
		slog.Info("Sending message")
		err := dbp.SendMessage(tps, defaults.MakeAuthorityRef(settings.ServerName), message, settings.Context)
		if err != nil {
			slog.Error("Send message failed:", slog.Any("err", err))
		} else {
			slog.Info("Send message succeeded")
		}
	}
}
