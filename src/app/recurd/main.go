package main

import (
	"flag"
	"log"
	"os"

	"github.com/directedbits/recur/src/app/recurd/daemon"
	statejsonfile "github.com/directedbits/recur/src/infra/jsonfile/state"
	processos "github.com/directedbits/recur/src/infra/os/process"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
)

func main() {
	configPathArg := flag.String("config", "", "Path to config file (default: ~/.config/recur/config.yaml)")
	socketArg := flag.String("socket", "", "Daemon address: Unix socket path or TCP host:port")
	logLevelArg := flag.String("log-level", "", "Log level: debug, info, warn, error (overrides config)")
	flag.Parse()

	configPath := strPtrOrNil(*configPathArg)
	socket := strPtrOrNil(*socketArg)
	logLevel := strPtrOrNil(*logLevelArg)

	store, cfgPath, err := configyaml.InitStore(configPath, &configyaml.Config{
		LogLevel:      logLevel,
		SocketAddress: socket,
	})
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	pidPath, err := processos.PIDPath()
	if err != nil {
		log.Fatalf("could not determine PID path: %v", err)
	}

	statePath, err := statejsonfile.DefaultPath()
	if err != nil {
		log.Fatalf("could not determine state path: %v", err)
	}

	effective := store.Get()
	sockAddr := ""
	if effective.SocketAddress != nil {
		sockAddr = *effective.SocketAddress
	}

	d := daemon.New(store, pidPath, sockAddr)
	d.SetConfigPath(cfgPath)
	d.SetStatePath(statePath)
	d.SetLaunchArgs(&statejsonfile.LaunchArgs{
		ConfigPath:    cfgPath,
		SocketAddress: sockAddr,
		LogLevel:      derefStr(logLevel),
	})
	if err := d.Run(); err != nil {
		log.Fatalf("daemon error: %v", err)
		os.Exit(1)
	}
}

func derefStr(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
