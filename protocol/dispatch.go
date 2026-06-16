package protocol

import (
	"context"
	"errors"
	"io"

	"github.com/bintoca/dbuf-demo-go/basic"
)

type RequestResponse struct {
	RequestReader                io.Reader
	RequestStreamGroup           uint32
	RequestHeaderParams          HeaderParams
	RequestHeaderStoreProperties HeaderStoreProperties
	RequestPathProperties        PathProperties
	HeaderStoreCache             *HeaderStoreCache
	ResponseHeaders              basic.Optional[basic.DbufValSeq]
	ResponseBody                 basic.Optional[basic.DbufValSeq]
	ResponseReader               io.Reader
	ResponseError                error
}

func InitRequest(rr *RequestResponse, headerInitial basic.InitialDecode, ctx context.Context) (err error) {
	rr.RequestHeaderParams, err = GetHeaderParams(rr.RequestReader, headerInitial)
	if err != nil {
		return err
	}
	rr.RequestHeaderStoreProperties, err = GetHeaderStoreProperties(rr.HeaderStoreCache, rr.RequestHeaderParams.Header, rr.RequestHeaderParams.HeaderDetails, ctx)
	if err != nil {
		return err
	}
	return nil
}

func Execute(r io.Reader, w io.Writer, tc *HeaderStoreCache, routeFunc func(rr *RequestResponse, ctx context.Context) error, ctx context.Context) (streamGroup uint32, err error) {
	streamGroup, firstByte, useFirstByte, err := ReadStreamGroup(r)
	if err != nil {
		return
	}
	if streamGroup != 0 {
		return streamGroup, basic.DataError(basic.Registry(Registry_stream_group), basic.DataValueNotAccepted())
	}
	firstRequest := true
	for {
		rr := RequestResponse{RequestReader: r, HeaderStoreCache: tc, RequestStreamGroup: streamGroup}
		headerInitial, er := ReadHeaderInitial(r, firstByte, useFirstByte && firstRequest)
		if er != nil {
			if er == io.EOF {
				break
			}
			return streamGroup, er
		}
		err = InitRequest(&rr, headerInitial, ctx)
		if err != nil {
			return
		}
		rr.RequestPathProperties, err = GetPathProperties(rr.RequestHeaderStoreProperties)
		if err != nil {
			rr.ResponseError = err
		}
		err = routeFunc(&rr, ctx)
		if err != nil {
			return
		}
		if w != nil {
			err = Respond(&rr, w)
			if err != nil {
				return
			}
		}
		firstRequest = false
	}
	return
}
func EncodeWrite(w io.Writer, v basic.DbufValSeq) error {
	b, err := basic.EncodeFullSeq(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func Respond(rr *RequestResponse, w io.Writer) (err error) {
	if !rr.ResponseHeaders.HasValue {
		return errors.New("Request not handled")
	}
	err = EncodeWrite(w, rr.ResponseHeaders.Value)
	if err != nil {
		return
	}
	if rr.ResponseBody.HasValue {
		err = EncodeWrite(w, rr.ResponseBody.Value)
		if err != nil {
			return
		}
	}
	if rr.ResponseReader != nil {
		_, err := io.Copy(w, rr.ResponseReader)
		if err != nil {
			return err
		}
	}
	return
}

func MakeErrorOnOtherStream(streamID uint64, v basic.DbufValSeq) basic.DbufValSeq {
	return basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{basic.Registry(Registry_reference).ToDbufSeq(), basic.Registry(basic.Registry_error).ToDbufSeq(),
		basic.Registry(Registry_stream_id).ToDbufSeq(), basic.Uint64(streamID).ToDbufSeq(),
		basic.Registry(basic.Registry_error).ToDbufSeq(), v}}
}
func ValueHeader(v basic.DbufValSeq) basic.Optional[basic.DbufValSeq] {
	return basic.Optional[basic.DbufValSeq]{HasValue: true, Value: basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{{Val: basic.Registry(basic.Registry_value)}, v}}}
}
func ErrorHeader(er error) basic.Optional[basic.DbufValSeq] {
	return basic.Optional[basic.DbufValSeq]{HasValue: true, Value: basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{{Val: basic.Registry(basic.Registry_error)}, basic.ErrorToVal(er)}}}
}
func EmptyHeader() basic.Optional[basic.DbufValSeq] {
	return basic.Optional[basic.DbufValSeq]{HasValue: true, Value: basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{}}}
}
