package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// GobCodec 编解码的结构体，需要实现Codec接口。
type GobCodec struct {
	conn io.ReadWriteCloser // 建立socket时的链接实例
	buf  *bufio.Writer      // 写缓冲，防止阻塞
	dec  *gob.Decoder       // 解码
	enc  *gob.Encoder       // 编码
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn), // 从conn中取数据进行解码
		enc:  gob.NewEncoder(buf),  // 编码后的数据存入buf，Encode(*)内部实现了存逻辑
	}
}

func (g *GobCodec) Close() error {
	return g.conn.Close()
}

// ReadHeader 解码后的数据存入h
func (g *GobCodec) ReadHeader(h *Header) error {
	return g.dec.Decode(h)
}

// ReadBody 解码后的数据存入body
func (g *GobCodec) ReadBody(body interface{}) error {
	return g.dec.Decode(body)
}

func (g *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = g.buf.Flush()
		if err != nil {
			_ = g.Close()
		}
	}()
	if err = g.enc.Encode(h); err != nil {
		log.Println("rpc: gob error encoding header:", err)
		return
	}
	if err = g.enc.Encode(body); err != nil {
		log.Println("rpc: gob error encoding body:", err)
		return
	}
	return nil
}

var _ Codec = (*GobCodec)(nil)
