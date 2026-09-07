package main

// xray-vless: minimal VLESS+REALITY server entry point.
// Subcommands: run (default), x25519, uuid, version.
// Behavior of x25519 / uuid is identical to the full Xray-core implementation
// (copied verbatim from main/commands/all/{x25519,curve25519,uuid}.go).

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf/serial"
	"lukechampine.com/blake3"

	// Feature/app registration (init() constructors required by core.New).
	_ "github.com/xtls/xray-core/app/log"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	_ "github.com/xtls/xray-core/proxy/blackhole"
	_ "github.com/xtls/xray-core/proxy/freedom"
	_ "github.com/xtls/xray-core/proxy/vless/inbound"
	_ "github.com/xtls/xray-core/transport/internet/tcp"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0][0] != '-' {
		switch args[0] {
		case "run":
			cmdRun(args[1:])
			return
		case "init":
			cmdInit(args[1:])
			return
		case "x25519":
			cmdX25519(args[1:])
			return
		case "uuid":
			cmdUUID(args[1:])
			return
		case "version":
			fmt.Println("Xray-min", core.Version(), "(VLESS + REALITY server, trimmed from Xray-core)")
			return
		case "help", "-h", "--help":
			usage()
			return
		default:
			fmt.Println("unknown command:", args[0])
			usage()
			os.Exit(2)
		}
	}
	cmdRun(args)
}

func usage() {
	fmt.Print(`Xray-min is a trimmed Xray-core: VLESS inbound + REALITY + freedom/blackhole outbound.

Usage:

  xray-vless init                    Interactive wizard: generate vless.json + import link
  xray-vless run -c config.json     Run the server (default command)
  xray-vless x25519                 Generate REALITY x25519 key pair
  xray-vless uuid [-i "example"]    Generate UUIDv4 or UUIDv5 (VLESS)
  xray-vless version                Show version

`)
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configFile := fs.String("c", "", "Config path for Xray.")
	fs.StringVar(configFile, "config", "", "Config path for Xray.")
	fs.Parse(args)

	file := *configFile
	if file == "" {
		if wd, err := os.Getwd(); err == nil {
			if _, err := os.Stat(wd + "/config.json"); err == nil {
				file = wd + "/config.json"
			}
		}
	}

	var config *core.Config
	var err error
	if file == "" {
		config, err = serial.LoadJSONConfig(os.Stdin)
	} else {
		f, ferr := os.Open(file)
		if ferr != nil {
			fmt.Println("Failed to open config file:", ferr)
			os.Exit(23)
		}
		defer f.Close()
		config, err = serial.LoadJSONConfig(f)
	}
	if err != nil {
		fmt.Println("Failed to load config:", err)
		os.Exit(23)
	}

	server, err := core.New(config)
	if err != nil {
		fmt.Println("Failed to create server:", err)
		os.Exit(23)
	}
	if err := server.Start(); err != nil {
		fmt.Println("Failed to start:", err)
		os.Exit(-1)
	}
	defer server.Close()

	// Explicitly triggering GC to remove garbage from config loading.
	runtime.GC()
	debug.FreeOSMemory()

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	<-osSignals
}

// ---- x25519 (identical behavior to full Xray-core) ----

func cmdX25519(args []string) {
	fs := flag.NewFlagSet("x25519", flag.ExitOnError)
	stdEncoding := fs.Bool("std-encoding", false, "")
	input := fs.String("i", "", "")
	fs.Parse(args)

	Curve25519Genkey(*stdEncoding, *input)
}

// Curve25519Genkey is copied verbatim from main/commands/all/curve25519.go.
func Curve25519Genkey(StdEncoding bool, input_base64 string) {
	var encoding *base64.Encoding
	if StdEncoding {
		encoding = base64.StdEncoding
	} else {
		encoding = base64.RawURLEncoding
	}

	var privateKey []byte
	if len(input_base64) > 0 {
		privateKey, _ = encoding.DecodeString(input_base64)
		if len(privateKey) != 32 {
			fmt.Println("Invalid length of X25519 private key.")
			return
		}
	}
	privateKey, password, hash32, err := genCurve25519(privateKey)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("PrivateKey: %v\nPassword (PublicKey): %v\nHash32: %v\n",
		encoding.EncodeToString(privateKey),
		encoding.EncodeToString(password),
		encoding.EncodeToString(hash32[:]))
}

func genCurve25519(inputPrivateKey []byte) (privateKey []byte, password []byte, hash32 [32]byte, returnErr error) {
	if len(inputPrivateKey) > 0 {
		privateKey = inputPrivateKey
	}
	if privateKey == nil {
		privateKey = make([]byte, 32)
		rand.Read(privateKey)
	}

	// Modify random bytes using algorithm described at:
	// https://cr.yp.to/ecdh.html
	// (Just to make sure printing the real private key)
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	key, err := ecdh.X25519().NewPrivateKey(privateKey)
	if err != nil {
		returnErr = err
		return
	}
	password = key.PublicKey().Bytes()
	hash32 = blake3.Sum256(password)
	return
}

// ---- uuid (identical behavior to full Xray-core) ----

func cmdUUID(args []string) {
	fs := flag.NewFlagSet("uuid", flag.ExitOnError)
	input := fs.String("i", "", "")
	fs.Parse(args)

	var output string
	if l := len(*input); l == 0 {
		u := uuid.New()
		output = u.String()
	} else if l <= 30 {
		u, _ := uuid.ParseString(*input)
		output = u.String()
	} else {
		output = "Input must be within 30 bytes."
	}
	fmt.Println(output)
}
