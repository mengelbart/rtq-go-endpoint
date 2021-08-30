package transport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/qlog"
	"github.com/mengelbart/rtq-go"
	"github.com/mengelbart/rtq-go-endpoint/internal/utils"
	"github.com/pion/rtcp"
)

type QUIC struct {
	session *rtq.Session
}

func NewQUICServer(addr string) (*QUIC, error) {
	quicConf := &quic.Config{
		EnableDatagrams: true,
	}
	qlogWriter, err := utils.GetQLOGWriter()
	if err != nil {
		return nil, fmt.Errorf("could not get qlog writer: %w", err)
	}
	if qlogWriter != nil {
		quicConf.Tracer = qlog.NewTracer(qlogWriter)
	}

	listener, err := quic.ListenAddr(addr, generateTLSConfig(), quicConf)
	if err != nil {
		return nil, err
	}
	quicSession, err := listener.Accept(context.Background())
	if err != nil {
		return nil, err
	}

	rtqSession, err := rtq.NewSession(quicSession)
	if err != nil {
		return nil, err
	}
	return &QUIC{
		session: rtqSession,
	}, nil
}

func NewQUICClient(addr string) (*QUIC, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"rtq"},
	}
	quicConf := &quic.Config{
		EnableDatagrams: true,
	}
	qlogWriter, err := utils.GetQLOGWriter()
	if err != nil {
		return nil, fmt.Errorf("could not get qlog writer: %w", err)
	}
	if qlogWriter != nil {
		quicConf.Tracer = qlog.NewTracer(qlogWriter)
	}
	quicSession, err := quic.DialAddr(addr, tlsConfig, quicConf)
	if err != nil {
		return nil, err
	}
	rtqSession, err := rtq.NewSession(quicSession)
	if err != nil {
		return nil, err
	}
	return &QUIC{
		session: rtqSession,
	}, nil
}

type WriteFlowCloser struct {
	*QUIC
	*rtq.WriteFlow
}

func (q *WriteFlowCloser) WriteRTCP(pkts []rtcp.Packet) (int, error) {
	buf, err := rtcp.Marshal(pkts)
	if err != nil {
		return 0, err
	}
	return q.Write(buf)
}

func (q *QUIC) Writer(id uint64) (*WriteFlowCloser, error) {
	f, err := q.session.OpenWriteFlow(id)
	if err != nil {
		return nil, err
	}
	return &WriteFlowCloser{
		QUIC:      q,
		WriteFlow: f,
	}, nil
}

type ReadFlowCloser struct {
	*QUIC
	*rtq.ReadFlow
}

func (q *QUIC) Reader(id uint64) (*ReadFlowCloser, error) {
	f, err := q.session.AcceptFlow(id)
	if err != nil {
		return nil, err
	}
	return &ReadFlowCloser{
		QUIC:     q,
		ReadFlow: f,
	}, nil
}

func (q *QUIC) Close() error {
	return q.session.Close()
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
		NextProtos:   []string{"rtq"},
	}
}