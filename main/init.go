package main

// xray-vless init: interactive wizard that generates a vless.json config
// and prints a VLESS one-click import link.

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"

	xuuid "github.com/xtls/xray-core/common/uuid"
)

var defaultSNIs = []string{
	"www.microsoft.com",
	"www.apple.com",
	"www.icloud.com",
	"www.amazon.com",
	"dl.google.com",
	"www.samsung.com",
}

type clientSetting struct {
	ID   string `json:"id"`
	Flow string `json:"flow,omitempty"`
}

type inboundSettings struct {
	Clients    []clientSetting `json:"clients"`
	Decryption string          `json:"decryption"`
}

type realitySettings struct {
	Show        bool     `json:"show"`
	Dest        string   `json:"dest"`
	ServerNames []string `json:"serverNames"`
	PrivateKey  string   `json:"privateKey"`
	ShortIds    []string `json:"shortIds"`
}

type streamSettings struct {
	Network         string           `json:"network"`
	Security        string           `json:"security"`
	RealitySettings *realitySettings `json:"realitySettings,omitempty"`
}

type inbound struct {
	Listen         string          `json:"listen"`
	Port           int             `json:"port"`
	Protocol       string          `json:"protocol"`
	Settings       inboundSettings `json:"settings"`
	StreamSettings streamSettings  `json:"streamSettings"`
}

type outbound struct {
	Protocol string `json:"protocol"`
	Tag      string `json:"tag"`
}

type logConfig struct {
	Loglevel string `json:"loglevel"`
}

type config struct {
	Log       logConfig  `json:"log"`
	Inbounds  []inbound  `json:"inbounds"`
	Outbounds []outbound `json:"outbounds"`
}

func cmdInit(args []string) {
	scanner := bufio.NewScanner(os.Stdin)

	ask := func(prompt, def string) string {
		if def != "" {
			fmt.Printf("%s [%s]: ", prompt, def)
		} else {
			fmt.Printf("%s: ", prompt)
		}
		if !scanner.Scan() {
			return def
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return def
		}
		return line
	}

	fmt.Println("=== xray-vless init: VLESS config wizard ===")

	// 1. Listen IP (default: all interfaces)
	ip := ask("Listen IP", "0.0.0.0")

	// 2. Port (default: 443)
	var port int
	for {
		line := ask("Listen port", "443")
		p, err := strconv.Atoi(line)
		if err == nil && p > 0 && p < 65536 {
			port = p
			break
		}
		fmt.Println("Invalid port, please enter a number between 1 and 65535.")
	}

	// 3. UUID (user input or auto-generated)
	var uid string
	for {
		line := ask("User UUID (empty to auto-generate)", "")
		if line == "" {
			u := xuuid.New()
			uid = u.String()
			break
		}
		if _, err := xuuid.ParseString(line); err == nil {
			uid = line
			break
		}
		fmt.Println("Invalid UUID, please try again.")
	}

	// 4. Use REALITY?
	useReality := false
	for {
		line := ask("Use REALITY? (y/n)", "y")
		if line == "y" || line == "Y" || line == "yes" || line == "Yes" {
			useReality = true
			break
		}
		if line == "n" || line == "N" || line == "no" || line == "No" {
			break
		}
		fmt.Println("Please answer y or n.")
	}

	var privKey, pubKey, sni, shortID string

	if useReality {
		// 5. x25519 key pair: user-provided private key or auto-generated
		for {
			line := ask("REALITY private key (empty to auto-generate x25519)", "")
			if line == "" {
				priv, pub, _, _ := genCurve25519(nil)
				privKey = base64.RawURLEncoding.EncodeToString(priv)
				pubKey = base64.RawURLEncoding.EncodeToString(pub)
				break
			}
			var raw []byte
			if b, err := base64.RawURLEncoding.DecodeString(line); err == nil {
				raw = b
			} else if b, err := base64.StdEncoding.DecodeString(line); err == nil {
				raw = b
			}
			if len(raw) != 32 {
				fmt.Println("Invalid private key length, need 32 bytes of base64.")
				continue
			}
			priv, pub, _, _ := genCurve25519(raw)
			privKey = base64.RawURLEncoding.EncodeToString(priv)
			pubKey = base64.RawURLEncoding.EncodeToString(pub)
			break
		}

		// 6. SNI: user input or random from defaults
		sni = ask(fmt.Sprintf("SNI server name (empty for random, e.g. %s)", defaultSNIs[0]), "")
		if sni == "" {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(defaultSNIs))))
			sni = defaultSNIs[n.Int64()]
			fmt.Printf("SNI selected: %s\n", sni)
		}

		// 7. shortId: auto-generated (8 random bytes = 16 hex chars)
		shortID = genShortID()
	}

	cfg := config{
		Log: logConfig{Loglevel: "warning"},
		Inbounds: []inbound{{
			Listen:   ip,
			Port:     port,
			Protocol: "vless",
			Settings: inboundSettings{
				Clients:    []clientSetting{{ID: uid}},
				Decryption: "none",
			},
			StreamSettings: streamSettings{
				Network:  "tcp",
				Security: "none",
			},
		}},
		Outbounds: []outbound{
			{Protocol: "freedom", Tag: "direct"},
			{Protocol: "blackhole", Tag: "block"},
		},
	}

	link := fmt.Sprintf("vless://%s@%s:%d?type=tcp&security=none&encryption=none#xray-vless", uid, ip, port)

	if useReality {
		cfg.Inbounds[0].Settings.Clients[0].Flow = "xtls-rprx-vision"
		cfg.Inbounds[0].StreamSettings.Security = "reality"
		cfg.Inbounds[0].StreamSettings.RealitySettings = &realitySettings{
			Show:        false,
			Dest:        sni + ":443",
			ServerNames: []string{sni},
			PrivateKey:  privKey,
			ShortIds:    []string{shortID},
		}
		link = fmt.Sprintf("vless://%s@%s:%d?type=tcp&security=reality&encryption=none&pbk=%s&sni=%s&sid=%s&fp=chrome&flow=xtls-rprx-vision#xray-vless",
			uid, ip, port, pubKey, sni, shortID)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Println("Failed to marshal config:", err)
		os.Exit(1)
	}
	if err := os.WriteFile("vless.json", data, 0644); err != nil {
		fmt.Println("Failed to write vless.json:", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("=== Done: config written to vless.json ===")
	fmt.Println("Run the server:  xray-vless run -c vless.json")
	fmt.Println("Import link (VLESS):")
	fmt.Println(link)
}

func genShortID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
