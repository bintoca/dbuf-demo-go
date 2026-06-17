package main

import (
	"github.com/bintoca/dbuf-demo-go/demo-server/defaults"
)

func main() {
	settings := defaults.DefaultSettings()
	routeFunc, storage := defaults.DefaultRouteFunc(settings.StoragePath, settings.ServerHostName)
	defer storage.Close()
	settings.Config.RouteFuncBi = routeFunc
	settings.Config.Storage = storage
	defaults.DemoServer(settings)
}
