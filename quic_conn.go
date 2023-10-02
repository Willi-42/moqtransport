package moqtransport

import (
	"context"

	"github.com/quic-go/quic-go"
)

type quicConn struct {
	conn quic.Connection
}

func (c *quicConn) OpenStream() (stream, error) {
	return c.conn.OpenStream()
}

func (c *quicConn) OpenStreamSync(ctx context.Context) (stream, error) {
	return c.conn.OpenStreamSync(ctx)
}

func (c *quicConn) OpenUniStream() (sendStream, error) {
	return c.conn.OpenUniStream()
}

func (c *quicConn) OpenUniStreamSync(ctx context.Context) (sendStream, error) {
	return c.conn.OpenUniStreamSync(ctx)
}

func (c *quicConn) AcceptStream(ctx context.Context) (stream, error) {
	return c.conn.AcceptStream(ctx)
}

func (c *quicConn) AcceptUniStream(ctx context.Context) (readStream, error) {
	return c.conn.AcceptUniStream(ctx)
}

func (c *quicConn) ReceiveMessage(ctx context.Context) ([]byte, error) {
	return c.conn.ReceiveMessage(ctx)
}