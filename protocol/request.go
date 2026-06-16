package protocol

import (
	"context"
	"fmt"
	"io"

	"github.com/bintoca/dbuf-demo-go/basic"
)

type Request struct {
	StreamGroup uint64
	Header      basic.DbufValSeq
	Body        io.Reader
}
type Response struct {
	HeaderStoreProperties HeaderStoreProperties
	Body                  io.Reader
}
type Connection interface {
	OpenStreamSync(ctx context.Context) (io.ReadWriteCloser, error)
	HeaderStoreCache() *HeaderStoreCache
}

func WriteRequest(w io.Writer, req Request) error {
	if req.StreamGroup > 0 {
		err := EncodeWrite(w, basic.Uint64(req.StreamGroup).ToDbufSeq())
		if err != nil {
			return err
		}
	}
	err := EncodeWrite(w, req.Header)
	if err != nil {
		return err
	}
	if req.Body != nil {
		_, err := io.Copy(w, req.Body)
		if err != nil {
			return err
		}
	}
	return nil
}
func ReadResponse(r io.Reader, tc *HeaderStoreCache, ctx context.Context) (tp HeaderStoreProperties, err error) {
	initial, err := ReadHeaderInitial(r, 0, false)
	if err != nil {
		return
	}
	header, err := GetHeaderParams(r, initial)
	if err != nil {
		err = fmt.Errorf("Get header params failed: %w", err)
		return
	}
	return GetHeaderStoreProperties(tc, header.Header, header.HeaderDetails, ctx)
}
func SendRequest(conn Connection, req Request, ctx context.Context) (re Response, err error) {
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return
	}
	defer stream.Close()
	re.Body = stream
	err = WriteRequest(stream, req)
	if err != nil {
		return
	}
	re.HeaderStoreProperties, err = ReadResponse(stream, conn.HeaderStoreCache(), ctx)
	if err != nil {
		err = fmt.Errorf("Read response failed: %w", err)
	}
	return
}
func SendRequestCheckError(conn Connection, req Request, ctx context.Context) (Response, error) {
	re, err := SendRequest(conn, req, ctx)
	if err != nil {
		return re, err
	}
	if ree, exists := re.HeaderStoreProperties.Headers[basic.Registry(basic.Registry_error).ToDbufValString()]; exists {
		return re, basic.ErrorBase([]basic.DbufValSeq{ree.ToDbufSeq()}, nil)
	}
	return re, err
}
func NotAuthenticated() basic.DbufError {
	return basic.ErrorBase([]basic.DbufValSeq{basic.Registry(Registry_not_authenticated).ToDbufSeq()}, nil)
}
func CreateAuthorityRef(host string) basic.DbufValSeq {
	return basic.DbufValSeq{Val: basic.Array(), Sequence: []basic.DbufValSeq{basic.Registry(Registry_host).ToDbufSeq(), basic.Text(host).ToDbufSeq(), basic.Registry(Registry_authority_marker).ToDbufSeq()}}
}
func CreateRequest(headerSequence []basic.DbufValSeq, body io.Reader) Request {
	return Request{Header: basic.DbufValSeq{Val: basic.Map(), Sequence: headerSequence}, Body: body}
}
