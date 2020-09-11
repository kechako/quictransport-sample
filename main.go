package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

type origins []string

func (o *origins) Set(value string) error {
	*o = append(*o, value)
	return nil
}

func (o *origins) String() string {
	return fmt.Sprint(*o)
}

func main() {
	var addr string
	var certFile string
	var keyFile string
	var allowedOrigins origins = origins{"localhost"}
	var debug bool
	flag.StringVar(&addr, "addr", "0.0.0.0:4433", "address to bind")
	flag.StringVar(&certFile, "cert", "cert.pem", "TLS certificate file.")
	flag.StringVar(&keyFile, "key", "key.pem", "TLS private key file.")
	flag.Var(&allowedOrigins, "allowed", "Allowed origins")
	flag.BoolVar(&debug, "debug", false, "Show debug message.")
	flag.Parse()

	level := Info
	if debug {
		level = Debug
	}
	var logger = NewStdLogger(level)

	server := &Server{
		Addr:           addr,
		CertFile:       certFile,
		KeyFile:        keyFile,
		Logger:         logger,
		AllowedOrigins: []string(allowedOrigins),
	}

	if err := server.Serve(context.Background()); err != nil {
		logger.Error(err)
		os.Exit(1)
	}
}
