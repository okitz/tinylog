//go:build pico || pico_w

// This file includes code derived from sources in github.com/soypat/cyw43439/examples/mqtt/main.go,
// which is licensed under the MIT License.
// Copyright (c) 2022 Patricio Whittingslow

package pico

import (
	"log/slog"
	"math/rand"
	"net/netip"
	"time"

	"github.com/soypat/seqs"
	"github.com/soypat/seqs/stacks"
)

const connTimeout = 5 * time.Second
const tcpbufsize = 2030 // MTU - ethhdr - iphdr - tcphdr
// Set this address to the server's address.
// You may run a local comqtt server: https://github.com/wind-c/comqtt
// build cmd/single, run it and change the IP address to your local server.

func SetupWifi(stack *stacks.PortStack, logger *slog.Logger, brokerAddr string, readyCh, disconnectCh chan struct{}) {
	start := time.Now()
	svAddr, err := netip.ParseAddrPort(brokerAddr)
	if err != nil {
		panic("parsing server address:" + err.Error())
	}
	// Resolver router's hardware address to dial outside our network to internet.
	serverHWAddr, err := ResolveHardwareAddr(stack, svAddr.Addr())
	if err != nil {
		panic("router hwaddr resolving:" + err.Error())
	}

	rng := rand.New(rand.NewSource(int64(time.Now().Sub(start))))
	// Start TCP server.
	clientAddr := netip.AddrPortFrom(stack.Addr(), uint16(rng.Intn(65535-1024)+1024))
	conn, err := stacks.NewTCPConn(stack, stacks.TCPConnConfig{
		TxBufSize: tcpbufsize,
		RxBufSize: tcpbufsize,
	})

	if err != nil {
		panic("conn create:" + err.Error())
	}

	closeConn := func(err string) {
		slog.Error("tcpconn:closing", slog.String("err", err))
		conn.Close()
		for !conn.State().IsClosed() {
			slog.Info("tcpconn:waiting", slog.String("state", conn.State().String()))
			time.Sleep(1000 * time.Millisecond)
		}
	}

	for {
		random := rng.Uint32()
		logger.Info("socket:listen")
		err = conn.OpenDialTCP(clientAddr.Port(), serverHWAddr, svAddr, seqs.Value(random))
		if err != nil {
			panic("socket dial:" + err.Error())
		}
		retries := 50
		for conn.State() != seqs.StateEstablished && retries > 0 {
			time.Sleep(100 * time.Millisecond)
			retries--
		}
		if retries == 0 {
			logger.Info("socket:no-establish")
			closeConn("did not establish connection")
			continue
		}

		logger.Info("socket:established",
			slog.String("state", conn.State().String()),
			slog.String("local", conn.LocalAddr().String()),
			slog.String("remote", conn.RemoteAddr().String()),
		)
		readyCh <- struct{}{}
		for {
			select {
			case <-disconnectCh:
				logger.Info("socket:disconnecting")
				closeConn("disconnected")
				return
			}
		}
	}
}
