package protocol

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bintoca/dbuf-demo-go/basic"
)

type ReceiveMessageFunc func(message basic.DbufVal, authenticatedIdentity basic.DbufValString) error

func DeliverMessage(rr *RequestResponse, ctx context.Context, receiveMessage ReceiveMessageFunc, maxMessageSize int) error {
	if !rr.RequestHeaderStoreProperties.CacheItem.IsAuthenticated {
		rr.ResponseError = NotAuthenticated()
		return nil
	}
	body, err := GetSingleBody(rr.RequestReader, rr.RequestHeaderStoreProperties, maxMessageSize)
	if err != nil {
		rr.ResponseError = err
		return nil
	}
	err = receiveMessage(body, rr.RequestHeaderStoreProperties.CacheItem.Identity())
	if err != nil {
		return fmt.Errorf("Receive message failed: %w", err)
	}
	rr.ResponseHeaders = EmptyHeader()
	return nil
}
func RoutePaths(rr *RequestResponse, receiveMessage ReceiveMessageFunc, ctx context.Context) {
	pp := rr.RequestPathProperties
	if pp.HasOperation {
		rr.ResponseError = basic.DataError(basic.Registry(Registry_header), basic.DataError(basic.Registry(Registry_operation), basic.DataKeyNotAccepted()))
	}
	if rr.ResponseError == nil {
		if len(pp.SubPath) > 0 && pp.SubPath[0].GetType() == basic.Parse_type_registry {
			switch pp.SubPath[0].GetValue() {
			case Registry_identity:
				if len(pp.SubPath) == 1 {
					err := CreateIdentity(rr, ctx)
					if err != nil {
						rr.ResponseError = fmt.Errorf("Create identity failed: %w", err)
					}
				}
			case Registry_deliver_message:
				if len(pp.SubPath) == 1 {
					err := DeliverMessage(rr, ctx, receiveMessage, SmallRequestBufferSize)
					if err != nil {
						rr.ResponseError = fmt.Errorf("Deliver message failed: %w", err)
					}
				}
			}
		}
	}
	if !rr.ResponseHeaders.HasValue && rr.ResponseError == nil {
		rr.ResponseError = errors.New("Request not handled")
	}
	if rr.ResponseError != nil {
		slog.Error("Response error", slog.Any("err", rr.ResponseError))
		rr.ResponseHeaders = ErrorHeader(rr.ResponseError)
	}
}
func CreateRequestForDeliverMessage(message basic.DbufValSeq, headerStore basic.DbufValSeq) Request {
	ref := basic.DbufValSeq{Val: basic.Array(), Sequence: []basic.DbufValSeq{basic.Registry(Registry_deliver_message).ToDbufSeq()}}
	return CreateRequest([]basic.DbufValSeq{basic.Registry(Registry_reference).ToDbufSeq(), ref,
		basic.Registry(Registry_header_store).ToDbufSeq(), headerStore,
		basic.Registry(basic.Registry_value).ToDbufSeq(), message}, nil)
}
func SendMessage(headerStoreState *HeaderStoreState, authorityRef basic.DbufValSeq, message basic.DbufValSeq, ctx context.Context) error {
	protectedHeaders := basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{
		basic.Registry(Registry_reference).ToDbufSeq(),
		authorityRef,
	}}
	tsh, err := headerStoreState.GetHeaderStoreHeader(protectedHeaders, ctx)
	if err != nil {
		return fmt.Errorf("Get header store header failed: %w", err)
	}
	_, err = SendRequestCheckError(headerStoreState.Connection, CreateRequestForDeliverMessage(message, tsh), ctx)
	if err != nil {
		return fmt.Errorf("Deliver message request failed: %w", err)
	}
	return nil
}
