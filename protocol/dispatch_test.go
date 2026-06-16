package protocol

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/bintoca/dbuf-demo-go/basic"
)

func TestExecute(t *testing.T) {
	ctx := context.Background()
	tc := MakeHeaderStoreCache(nil, nil)

	t.Run("InvalidStreamGroup", func(t *testing.T) {
		// Stream group 1 is encoded. Execute only accepts 0.
		sgData, _ := basic.EncodeFullSeq(basic.Uint64(1).ToDbufSeq())
		r := bytes.NewReader(sgData)

		sg, err := Execute(r, nil, tc, nil, ctx)
		if sg != 1 || err == nil {
			t.Errorf("Expected error for stream group 1, got sg=%d err=%v", sg, err)
		}
	})

	t.Run("RouteExecution", func(t *testing.T) {
		// Minimal data: stream group 0 + empty map header
		sgData, _ := basic.EncodeFullSeq(basic.Uint64(0).ToDbufSeq())
		headerData, _ := basic.EncodeFullSeq(basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{}})

		r := io.MultiReader(bytes.NewReader(sgData), bytes.NewReader(headerData))
		w := new(bytes.Buffer)

		called := false
		routeFunc := func(rr *RequestResponse, ctx context.Context) error {
			called = true
			rr.ResponseHeaders = EmptyHeader()
			return nil
		}

		_, err := Execute(r, w, tc, routeFunc, ctx)
		// Execute will loop until EOF
		if err != nil && err != io.EOF {
			t.Errorf("Execution failed: %v", err)
		}
		if !called {
			t.Error("Route function was not called")
		}
	})
}
