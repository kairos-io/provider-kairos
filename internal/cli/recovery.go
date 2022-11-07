package cli

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	edgevpnConfig "github.com/mudler/edgevpn/pkg/config"

	"github.com/ipfs/go-log"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/pterm/pterm"
)

func networkConfig(token, address, loglevel, i string) *edgevpnConfig.Config {
	return &edgevpnConfig.Config{
		NetworkToken:   token,
		Address:        address,
		Libp2pLogLevel: "error",
		FrameTimeout:   "30s",
		BootstrapIface: true,
		LogLevel:       loglevel,
		LowProfile:     true,
		VPNLowProfile:  true,
		Interface:      i,
		Concurrency:    runtime.NumCPU(),
		PacketMTU:      1420,
		InterfaceMTU:   1200,
		Ledger: edgevpnConfig.Ledger{
			AnnounceInterval: time.Duration(30) * time.Second,
			SyncInterval:     time.Duration(30) * time.Second,
		},
		NAT: edgevpnConfig.NAT{
			Service:           true,
			Map:               true,
			RateLimit:         true,
			RateLimitGlobal:   10,
			RateLimitPeer:     10,
			RateLimitInterval: time.Duration(10) * time.Second,
		},
		Discovery: edgevpnConfig.Discovery{
			DHT:      true,
			MDNS:     true,
			Interval: time.Duration(120) * time.Second,
		},
		Connection: edgevpnConfig.Connection{
			RelayV1: true,

			AutoRelay:      true,
			MaxConnections: 100,
			MaxStreams:     100,
			HolePunch:      true,
		},
	}
}

func startRecoveryService(ctx context.Context, token, name, address, loglevel string) error {

	nc := networkConfig(token, "", loglevel, "kairosrecovery0")

	lvl, err := log.LevelFromString(loglevel)
	if err != nil {
		lvl = log.LevelError
	}
	llger := logger.New(lvl)

	o, _, err := nc.ToOpts(llger)
	if err != nil {
		llger.Fatal(err.Error())
	}

	o = append(o,
		services.Alive(
			time.Duration(20)*time.Second,
			time.Duration(10)*time.Second,
			time.Duration(10)*time.Second)...)

	// opts, err := vpn.Register(vpnOpts...)
	// if err != nil {
	// 	return err
	// }
	o = append(o, services.RegisterService(llger, time.Duration(5*time.Second), name, address)...)

	e, err := node.New(o...)
	if err != nil {
		return err
	}

	return e.Start(ctx)
}

func sshServer(listenAdddr, password string) {
	ssh.Handle(func(s ssh.Session) {
		cmd := exec.Command("/bin/bash")
		ptyReq, winCh, isPty := s.Pty()
		if isPty {
			cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
			f, err := pty.Start(cmd)
			if err != nil {
				pterm.Warning.Println("Failed reserving tty")
			}
			go func() {
				for win := range winCh {
					setWinsize(f, win.Width, win.Height)
				}
			}()
			go func() {
				io.Copy(f, s) //nolint:errcheck
			}()
			io.Copy(s, f) //nolint:errcheck
			cmd.Wait()    //nolint:errcheck
		} else {
			io.WriteString(s, "No PTY requested.\n") //nolint:errcheck
			s.Exit(1)                                //nolint:errcheck
		}
	})

	pterm.Info.Println(ssh.ListenAndServe(listenAdddr, nil, ssh.PasswordAuth(func(ctx ssh.Context, pass string) bool {
		return pass == password
	}),
	))
}

func StartRecoveryService(tk, serviceUUID, generatedPassword, listenAddr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := startRecoveryService(ctx, tk, serviceUUID, listenAddr, "fatal"); err != nil {
		return err
	}

	sshServer(listenAddr, generatedPassword)

	return fmt.Errorf("should not return")
}
