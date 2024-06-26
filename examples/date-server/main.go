package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/mengelbart/moqtransport"
	"github.com/mengelbart/moqtransport/quicmoq"
	"github.com/mengelbart/moqtransport/webtransportmoq"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

func main() {
	certFile := flag.String("cert", "localhost.pem", "TLS certificate file")
	keyFile := flag.String("key", "localhost-key.pem", "TLS key file")
	addr := flag.String("addr", "localhost:8080", "listen address")
	flag.Parse()

	if err := run(context.Background(), *addr, *certFile, *keyFile); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, addr string, certFile, keyFile string) error {
	tlsConfig, err := generateTLSConfigWithCertAndKey(certFile, keyFile)
	if err != nil {
		log.Printf("failed to generate TLS config from cert file and key, generating in memory certs: %v", err)
		tlsConfig = generateTLSConfig()
	}
	return listen(ctx, addr, tlsConfig)
}

func listen(ctx context.Context, addr string, tlsConfig *tls.Config) error {
	listener, err := quic.ListenAddr(addr, tlsConfig, &quic.Config{
		EnableDatagrams: true,
	})
	if err != nil {
		return err
	}
	wt := webtransport.Server{
		H3: http3.Server{
			Addr:      addr,
			TLSConfig: tlsConfig,
		},
	}
	track := moqtransport.NewLocalTrack("clock", "second")
	defer track.Close()
	go func() {
		ticker := time.NewTicker(time.Second)
		id := uint64(0)
		for ts := range ticker.C {
			track.WriteObject(ctx, moqtransport.Object{
				GroupID:              id,
				ObjectID:             0,
				ObjectSendOrder:      0,
				ForwardingPreference: moqtransport.ObjectForwardingPreferenceStream,
				Payload:              []byte(fmt.Sprintf("%v", ts)),
			})
			id++
		}
	}()
	http.HandleFunc("/moq", func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			log.Printf("upgrading to webtransport failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		moqSession := &moqtransport.Session{
			Conn:            webtransportmoq.New(session),
			EnableDatagrams: true,
		}
		if err := moqSession.RunServer(ctx); err != nil {
			log.Printf("MoQ Session initialization failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Printf("got new session")
		go handle(moqSession, track)
	})
	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			return err
		}
		if conn.ConnectionState().TLS.NegotiatedProtocol == "h3" {
			go wt.ServeQUICConn(conn)
		}
		if conn.ConnectionState().TLS.NegotiatedProtocol == "moq-00" {
			s := &moqtransport.Session{
				Conn:            quicmoq.New(conn),
				EnableDatagrams: true,
			}
			if err := s.RunServer(ctx); err != nil {
				return err
			}
			go handle(s, track)
		}
	}
}

func handle(s *moqtransport.Session, t *moqtransport.LocalTrack) {
	if err := s.AddLocalTrack(t); err != nil {
		panic(err)
	}
	if err := s.Announce(context.Background(), "clock"); err != nil {
		panic(err)
	}
}

func generateTLSConfigWithCertAndKey(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"moq-00", "h3"},
	}, nil
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"moq-00", "h3"},
	}
}
