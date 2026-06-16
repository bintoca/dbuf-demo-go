package basic

import (
	"errors"
	"reflect"
	"testing"
)

type mockSimpleError struct {
	msg string
}

func (e mockSimpleError) Error() string { return e.msg }

func TestErrorToValInternal(t *testing.T) {
	t.Run("StandardErrorDefault", func(t *testing.T) {
		err := errors.New("standard error")
		got := ErrorToValInternal(err, nil)
		want := DbufValSeq{Val: Registry(Registry_error_internal)}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got.DebugString(), want.DebugString())
		}
	})

	t.Run("StandardErrorWithInternalFunc", func(t *testing.T) {
		err := errors.New("custom error")
		internalFunc := func(e error) DbufValSeq {
			return Text(e.Error()).ToDbufSeq()
		}
		got := ErrorToValInternal(err, internalFunc)
		want := Text("custom error").ToDbufSeq()

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got.DebugString(), want.DebugString())
		}
	})

	t.Run("DbufErrorSingleValue", func(t *testing.T) {
		err := IncompleteStream()
		got := ErrorToValInternal(err, nil)
		want := Registry(Registry_incomplete_stream).ToDbufSeq()

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got.DebugString(), want.DebugString())
		}
	})

	t.Run("DbufErrorMultiValueMap", func(t *testing.T) {
		// DataErrorSeq creates a DbufError with multiple registry values in a sequence
		path := Text("field_name").ToDbufSeq()
		err := DataErrorSeq(path, errors.New("inner"))
		
		got := ErrorToValInternal(err, nil)
		
		// Because it has multiple values and is wrapped, 
		// ErrorToValInternal should return a Map containing the values + the wrapped error
		if got.Val.GetType() != Parse_type_map {
			t.Errorf("Expected Map type for multi-value DbufError, got %v", got.Val.GetType())
		}
	})

	t.Run("NestedDbufErrorSingleWrap", func(t *testing.T) {
		inner := IncompleteStream()
		outer := TextError("outer error")
		wrappedErr := ErrorWrap(outer, inner)

		got := ErrorToValInternal(wrappedErr, nil)

		// Expected structure for nested single value:
		// Sequence[0]: Registry(Registry_value)
		// Sequence[1]: Text("outer error")
		// Sequence[2]: Registry(Registry_error)
		// Sequence[3]: Registry(Registry_incomplete_stream)
		
		if len(got.Sequence) != 4 {
			t.Fatalf("Expected sequence length 4, got %d", len(got.Sequence))
		}
		if !got.Sequence[3].Val.Equal(Registry(Registry_incomplete_stream)) {
			t.Errorf("Deepest nested value mismatch, got %v", got.Sequence[3].DebugString())
		}
	})

	t.Run("NestedDbufErrorMultiWrap", func(t *testing.T) {
		err1 := IncompleteStream()
		err2 := ErrorInternal()
		
		// Create a DbufError that wraps multiple errors
		outer := ErrorBase([]DbufValSeq{Text("multi-wrap").ToDbufSeq()}, []error{err1, err2})
		
		got := ErrorToValInternal(outer, nil)

		// Sequence[0]: Registry(Registry_value)
		// Sequence[1]: Text("multi-wrap")
		// Sequence[2]: Registry(Registry_error)
		// Sequence[3]: Array wrapping [err1, err2]
		
		if len(got.Sequence) != 4 {
			t.Fatalf("Expected sequence length 4, got %d", len(got.Sequence))
		}
		
		wrappedArray := got.Sequence[3]
		if wrappedArray.Val.GetType() != Parse_type_array {
			t.Errorf("Expected Array for multi-unwrap, got %v", wrappedArray.Val.GetType())
		}
		
		if len(wrappedArray.Sequence) != 2 {
			t.Errorf("Expected 2 wrapped errors in array, got %d", len(wrappedArray.Sequence))
		}
	})
}