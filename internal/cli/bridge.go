package cli

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ipfs/go-log"
	qr "github.com/kairos-io/go-nodepair/qrcode"
	"github.com/kairos-io/kairos-sdk/utils"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/mudler/edgevpn/api"
	"github.com/mudler/edgevpn/cmd"
	"github.com/mudler/edgevpn/pkg/config"
	"github.com/mudler/edgevpn/pkg/logger"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/vpn"
	"github.com/multiformats/go-multiaddr"
	"github.com/urfave/cli/v2"
)

func BridgeCMD(toolName string) *cli.Command {
	usage := "Connect to a kairos VPN network"
	description := `
		Starts a bridge with a kairos network or a node.

		# With a network

		By default, "bridge" will create a VPN network connection to the node with the token supplied, thus it requires elevated permissions in order to work.

		For example:

		$ sudo %s bridge --token <TOKEN>

		Will start a VPN, which local ip is fixed to 10.1.0.254 (tweakable with --address).

		The API will also be accessible at http://127.0.0.1:8080

		# With a node

		"%s bridge" can be used also to connect over to a node in recovery mode. When operating in this modality kairos bridge requires no specific permissions, indeed a tunnel
		will be created locally to connect to the machine remotely.

		For example:

		$ %s bridge --qr-code-image /path/to/image.png

		Will scan the QR code in the image and connect over. Further instructions on how to connect over will be printed out to the screen.

		See also: https://kairos.io/docs/reference/recovery_mode/

		`

	if toolName != "kairosctl" {
		usage += " (WARNING: this command will be deprecated in the next release, use the kairosctl binary instead)"
		description = "\t\tWARNING: This command will be deprecated in the next release. Please use the new kairosctl binary instead.\n" + description
	}

	flags := []cli.Flag{
		&cli.BoolFlag{
			Name:     "qr-code-snapshot",
			Required: false,
			Usage:    "Bool to take a local snapshot instead of reading from an image file for recovery",
			EnvVars:  []string{"QR_CODE_SNAPSHOT"},
		},
		&cli.StringFlag{
			Name:     "qr-code-image",
			Usage:    "Path to an image containing a valid QR code for recovery mode",
			Required: false,
			EnvVars:  []string{"QR_CODE_IMAGE"},
		},
		&cli.StringFlag{
			Name:  "api",
			Value: "127.0.0.1:8080",
			Usage: "Listening API url",
		},
		&cli.BoolFlag{
			Name:    "dhcp",
			EnvVars: []string{"DHCP"},
			Usage:   "Enable DHCP",
		},
		&cli.StringFlag{
			Value:   "10.1.0.254/24",
			Name:    "address",
			EnvVars: []string{"ADDRESS"},
			Usage:   "Specify an address for the bridge",
		},
		&cli.StringFlag{
			Value:   "/tmp/kairos",
			Name:    "lease-dir",
			EnvVars: []string{"lease-dir"},
			Usage:   "DHCP Lease directory",
		},
		&cli.StringFlag{
			Name:    "interface",
			Usage:   "Interface name",
			Value:   "kairos0",
			EnvVars: []string{"IFACE"},
		},
	}

	flags = append(flags, cmd.CommonFlags...)

	return &cli.Command{
		Name:        "bridge",
		UsageText:   fmt.Sprintf("%s %s", toolName, "bridge --token XXX"),
		Usage:       usage,
		Description: fmt.Sprintf(description, toolName, toolName, toolName),
		Flags:       flags,
		Action:      bridge,
	}
}

func stringsToMultiAddr(peers []string) []multiaddr.Multiaddr {
	res := []multiaddr.Multiaddr{}
	for _, p := range peers {
		addr, err := multiaddr.NewMultiaddr(p)
		if err != nil {
			continue
		}
		res = append(res, addr)
	}
	return res
}

func configFromContext(c *cli.Context) *config.Config {
	autorelayInterval, err := time.ParseDuration(c.String("autorelay-discovery-interval"))
	if err != nil {
		autorelayInterval = 0
	}
	var limitConfig *rcmgr.PartialLimitConfig
	d := map[string]map[string]interface{}{}

	return &config.Config{
		NetworkConfig:     c.String("config"),
		NetworkToken:      c.String("token"),
		Address:           c.String("address"),
		Router:            c.String("router"),
		Interface:         c.String("interface"),
		Libp2pLogLevel:    c.String("libp2p-log-level"),
		LogLevel:          c.String("log-level"),
		LowProfile:        c.Bool("low-profile"),
		Blacklist:         c.StringSlice("blacklist"),
		Concurrency:       c.Int("concurrency"),
		FrameTimeout:      c.String("timeout"),
		ChannelBufferSize: c.Int("channel-buffer-size"),
		InterfaceMTU:      c.Int("mtu"),
		PacketMTU:         c.Int("packet-mtu"),
		BootstrapIface:    c.Bool("bootstrap-iface"),
		Whitelist:         stringsToMultiAddr(c.StringSlice("whitelist")),
		Ledger: config.Ledger{
			StateDir:         c.String("ledger-state"),
			AnnounceInterval: time.Duration(c.Int("ledger-announce-interval")) * time.Second,
			SyncInterval:     time.Duration(c.Int("ledger-syncronization-interval")) * time.Second,
		},
		NAT: config.NAT{
			Service:           c.Bool("natservice"),
			Map:               c.Bool("natmap"),
			RateLimit:         c.Bool("nat-ratelimit"),
			RateLimitGlobal:   c.Int("nat-ratelimit-global"),
			RateLimitPeer:     c.Int("nat-ratelimit-peer"),
			RateLimitInterval: time.Duration(c.Int("nat-ratelimit-interval")) * time.Second,
		},
		Discovery: config.Discovery{
			BootstrapPeers: c.StringSlice("discovery-bootstrap-peers"),
			DHT:            c.Bool("dht"),
			MDNS:           c.Bool("mdns"),
			Interval:       time.Duration(c.Int("discovery-interval")) * time.Second,
		},
		Connection: config.Connection{
			AutoRelay:                  c.Bool("autorelay"),
			MaxConnections:             c.Int("max-connections"),
			HolePunch:                  c.Bool("holepunch"),
			StaticRelays:               c.StringSlice("autorelay-static-peer"),
			AutoRelayDiscoveryInterval: autorelayInterval,
			OnlyStaticRelays:           c.Bool("autorelay-static-only"),
			HighWater:                  c.Int("connection-high-water"),
			LowWater:                   c.Int("connection-low-water"),
		},
		Limit: config.ResourceLimit{
			Enable:      c.Bool("limit-enable"),
			FileLimit:   c.String("limit-file"),
			Scope:       c.String("limit-scope"),
			MaxConns:    c.Int("max-connections"), // Turn to 0 to use other way of limiting. Files take precedence
			LimitConfig: limitConfig,
		},
		PeerGuard: config.PeerGuard{
			Enable:        c.Bool("peerguard"),
			PeerGate:      c.Bool("peergate"),
			Relaxed:       c.Bool("peergate-relaxed"),
			Autocleanup:   c.Bool("peergate-autoclean"),
			SyncInterval:  time.Duration(c.Int("peergate-interval")) * time.Second,
			AuthProviders: d,
		},
	}
}

// bridge is just starting a VPN with edgevpn to the given network token.
func bridge(c *cli.Context) error {
	qrCodePath := ""
	fromQRCode := false
	var serviceUUID, sshPassword string

	if c.String("qr-code-image") != "" {
		qrCodePath = c.String("qr-code-image")
		fromQRCode = true
	}
	if c.Bool("qr-code-snapshot") {
		qrCodePath = ""
		fromQRCode = true
	}

	if fromQRCode {
		recoveryToken := qr.Reader(qrCodePath)
		data := utils.DecodeRecoveryToken(recoveryToken)
		if len(data) != 3 {
			fmt.Println("Token not decoded correctly")
			return fmt.Errorf("invalid token")
		}
		token := data[0]
		serviceUUID = data[1]
		sshPassword = data[2]
		if serviceUUID == "" || sshPassword == "" || token == "" {
			return fmt.Errorf("decoded invalid values")
		}

		c.Set("token", token)
	}

	ctx := context.Background()

	nc := configFromContext(c)

	lvl, err := log.LevelFromString(nc.LogLevel)
	if err != nil {
		lvl = log.LevelError
	}
	llger := logger.New(lvl)

	o, vpnOpts, err := nc.ToOpts(llger)
	if err != nil {
		llger.Fatal(err.Error())
	}

	opts := []node.Option{}

	if !fromQRCode {
		// We just connect to a VPN token
		o = append(o,
			services.Alive(
				time.Duration(20)*time.Second,
				time.Duration(10)*time.Second,
				time.Duration(10)*time.Second)...)

		if c.Bool("dhcp") {
			// Adds DHCP server
			address, _, err := net.ParseCIDR(c.String("address"))
			if err != nil {
				return err
			}
			nodeOpts, vO := vpn.DHCP(llger, 15*time.Minute, c.String("lease-dir"), address.String())
			o = append(o, nodeOpts...)
			vpnOpts = append(vpnOpts, vO...)
		}

		opts, err = vpn.Register(vpnOpts...)
		if err != nil {
			return err
		}
	} else {
		// We hook into a service
		llger.Info("Connecting to service", serviceUUID)
		llger.Info("SSH access password is", sshPassword)
		llger.Info("SSH server reachable at 127.0.0.1:2200")
		opts = append(opts, node.WithNetworkService(
			services.ConnectNetworkService(
				30*time.Second,
				serviceUUID,
				"127.0.0.1:2200",
			),
		))
		llger.Info("To connect, keep this terminal open and run in another terminal 'ssh 127.0.0.1 -p 2200' the password is ", sshPassword)
		llger.Info("Note: the connection might not be available instantly and first attempts will likely fail.")
		llger.Info("      Few attempts might be required before establishing a tunnel to the host.")
	}

	e, err := node.New(append(o, opts...)...)
	if err != nil {
		return err
	}

	go api.API(ctx, c.String("api"), 5*time.Second, 20*time.Second, e, nil, false) //nolint:errcheck

	return e.Start(ctx)
}
