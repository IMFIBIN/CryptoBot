package webserver

import (
	"cryptobot/internal/app/realflow"
	"cryptobot/internal/transport/httpapi"
)

func New(addr string) *httpapi.Server {
	return httpapi.New(addr, realflow.New())
}
