package main

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"strings"
	"sync"

	"github.com/lucas-clemente/quic-go"
)

const maxClientIndicationLength = 65535

type clientIndicationKey uint16

const (
	clientIndicationKeyOrigin clientIndicationKey = 0x0000
	clientIndicationKeyPath   clientIndicationKey = 0x0001
)

type ClientIndication struct {
	Origin string
	Path   string
}

type Server struct {
	Addr     string
	CertFile string
	KeyFile  string

	Logger Logger

	AllowedOrigins     []string
	allowedOriginTable map[string]struct{}
	once               sync.Once
}

func Serve(ctx context.Context, addr, certPath, keyPath string) error {
	s := &Server{
		Addr:     addr,
		CertFile: certPath,
		KeyFile:  keyPath,
	}

	return s.Serve(ctx)
}

func (s *Server) Serve(ctx context.Context) error {
	if s.Addr == "" {
		return errors.New("Server.Addr is not specified")
	}
	if s.CertFile == "" {
		return errors.New("Server.CertFile is not specified")
	}
	if s.KeyFile == "" {
		return errors.New("Server.KeyFile is not specified")
	}

	tlsConf, err := generateTLSConfig(s.CertFile, s.KeyFile)
	if err != nil {
		return err
	}

	listener, err := quic.ListenAddr(s.Addr, tlsConf, nil)
	if err != nil {
		return fmt.Errorf("failed to listen QUIC transport: %w", err)
	}

	if s.Logger == nil {
		s.Logger = nopLogger{}
	}

	for {
		sess, err := listener.Accept(ctx)
		if err != nil {
			return fmt.Errorf("failed to accept client connection: %w", err)
		}
		s.Logger.Infof("session accepted: %s", sess.RemoteAddr().String())
		go func() {
			defer func() {
				_ = sess.CloseWithError(0, "bye")
				s.Logger.Info("close session")
			}()
			s.handleSession(ctx, sess)
		}()
	}
}

const alpnQuicTransport = "wq-vvv-01"

func generateTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{alpnQuicTransport},
	}, nil
}

func (s *Server) handleSession(ctx context.Context, sess quic.Session) {
	s.Logger.Infof("TLS server name: %s", sess.ConnectionState().ServerName)
	stream, err := sess.AcceptUniStream(ctx)
	if err != nil {
		s.Logger.Errorf("failed to accept unidirectional stream: %v", err)
		return
	}
	s.Logger.Infof("unidirectional stream accepted, id: %d", stream.StreamID())

	indication, err := s.receiveClientIndication(stream)
	if err != nil {
		s.Logger.Error(err)
		return
	}
	s.Logger.Infof("client indication: %+v", indication)

	if err := s.validateClientIndication(indication); err != nil {
		s.Logger.Error(err)
		return
	}

	go func() {
		if err := s.communicateUni(ctx, sess); err != nil {
			s.Logger.Error(err)
		}
	}()

	if err := s.communicate(ctx, sess); err != nil {
		s.Logger.Error(err)
	}
}

func (s *Server) receiveClientIndication(stream quic.ReceiveStream) (indication ClientIndication, err error) {
	r := io.LimitReader(stream, maxClientIndicationLength)

	var buf [math.MaxUint16]byte
	for {
		var key clientIndicationKey
		err = binary.Read(r, binary.BigEndian, &key)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return
		}
		var length uint16
		err = binary.Read(r, binary.BigEndian, &length)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}

			return
		}
		_, err = io.ReadFull(r, buf[:length])
		if err != nil {
			return
		}

		value := string(buf[:length])

		switch key {
		case clientIndicationKeyOrigin:
			indication.Origin = value
		case clientIndicationKeyPath:
			indication.Path = value
		default:
			s.Logger.Warnf("skip unknown client indication key: %d: %s", key, value)
		}
	}
	err = nil

	return
}

var (
	errBadOrigin = errors.New("bad origin")
	errBadPath   = errors.New("bad path")
)

func (s *Server) validateClientIndication(indication ClientIndication) error {
	u, err := url.Parse(indication.Origin)
	if err != nil {
		return errBadOrigin
	}

	if indication.Path == "" {
		return errBadPath
	}

	u, err = u.Parse(indication.Path)
	if err != nil {
		return errBadPath
	}

	if !s.isOriginAllowd(u.Host) {
		return errBadOrigin
	}

	return nil
}

func (s *Server) communicate(ctx context.Context, sess quic.Session) error {
	for {
		stream, err := sess.AcceptStream(ctx)
		if err != nil {
			return fmt.Errorf("failed to accept bidirectional stream: %w", err)
		}
		s.Logger.Infof("bidirectional stream accepted: %d", stream.StreamID())
		if _, err := io.Copy(stream, stream); err != nil {
			return fmt.Errorf("failed to copy stream: %w", err)
		}
	}
}

func (s *Server) communicateUni(ctx context.Context, sess quic.Session) error {
	for {
		recvStream, err := sess.AcceptUniStream(ctx)
		if err != nil {
			return fmt.Errorf("failed to accept unidirectional stream: %w", err)
		}
		s.Logger.Infof("unidirectional stream accepted: %d", recvStream.StreamID())

		sendStream, err := sess.OpenUniStreamSync(ctx)
		if err != nil {
			return fmt.Errorf("failed to open unidirectional stream: %w", err)
		}
		s.Logger.Infof("unidirectional stream opened: %d", sendStream.StreamID())

		if _, err := io.Copy(sendStream, recvStream); err != nil {
			sendStream.Close()
			return fmt.Errorf("failed to copy stream: %w", err)
		}
		sendStream.Close()
	}
}

func (s *Server) isOriginAllowd(origin string) (allowed bool) {
	s.once.Do(func() {
		s.allowedOriginTable = make(map[string]struct{})
		for _, o := range s.AllowedOrigins {
			s.allowedOriginTable[strings.ToLower(o)] = struct{}{}
		}
	})

	_, allowed = s.allowedOriginTable[strings.ToLower(origin)]

	return
}
