package main

import (
	"context"
	"log/slog"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	t.Run("error - proxy unreachable", func(t *testing.T) {
		// given
		listenConfig := net.ListenConfig{}
		listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr, ok := listener.Addr().(*net.TCPAddr)
		require.True(t, ok)
		freePort := addr.Port
		require.NoError(t, listener.Close())

		configFile = "../configs/nonclave/notee.yaml"
		host = "127.0.0.1"
		port = freePort
		portTLS = freePort

		// when
		err = run(slog.New(slog.DiscardHandler))

		// then
		require.Error(t, err)
	})
}
