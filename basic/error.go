package basic

type DbufError interface {
	DbufError() []DbufValSeq
	Error() string
	Unwrap() []error
}

type DbufErrorVal struct {
	Val     []DbufValSeq
	Wrapped []error
}

func DbufErrorStringify(v DbufErrorVal) string {
	s := "Err:{"
	for _, v := range v.Val {
		s += v.ToString(DebugRegistryNames) + "\n"
	}
	s += "}\nWrapped:["
	for _, e := range v.Wrapped {
		s += e.Error() + "\n"
	}
	s += "]"
	return s
}

var DbufErrorStringFunc func(v DbufErrorVal) string

type DbufErrorHolder struct {
	msg string
	Val *DbufErrorVal
}

func (x DbufErrorHolder) DbufError() []DbufValSeq { return x.Val.Val }
func (x DbufErrorHolder) Error() string {
	return x.msg
}
func (x DbufErrorHolder) Unwrap() []error { return x.Val.Wrapped }

func (v DbufVal) ToDbufSeq() DbufValSeq {
	return DbufValSeq{Val: v}
}
func (v DbufValString) ToDbufSeq() DbufValSeq {
	return DbufValSeq{Val: DbufVal{Type: v.Type, Value: v.Value, Slice: []byte(v.String)}}
}
func ErrorBase(val []DbufValSeq, wrapped []error) DbufErrorHolder {
	v := DbufErrorVal{Val: val, Wrapped: wrapped}
	var m string
	if DbufErrorStringFunc != nil {
		m = DbufErrorStringFunc(v)
	} else {
		m = DbufErrorStringify(v)
	}
	return DbufErrorHolder{Val: &v, msg: m}
}
func IncompleteStream() DbufError {
	return ErrorBase([]DbufValSeq{Registry(Registry_incomplete_stream).ToDbufSeq()}, nil)
}
func ErrorInternal() DbufError {
	return ErrorBase([]DbufValSeq{Registry(Registry_error_internal).ToDbufSeq()}, nil)
}
func DataError(path DbufVal, err error) DbufError {
	return DataErrorSeq(path.ToDbufSeq(), err)
}
func DataErrorSeq(path DbufValSeq, err error) DbufError {
	return ErrorBase([]DbufValSeq{Registry(Registry_value).ToDbufSeq(), Registry(Registry_data_error).ToDbufSeq(), Registry(Registry_data_path).ToDbufSeq(), path}, []error{err})
}
func DataKeyNotAccepted() DbufError {
	return ErrorBase([]DbufValSeq{Registry(Registry_data_key_not_accepted).ToDbufSeq()}, nil)
}
func DataTypeNotAccepted() DbufError {
	return ErrorBase([]DbufValSeq{Registry(Registry_data_type_not_accepted).ToDbufSeq()}, nil)
}
func DataValueNotAccepted() DbufError {
	return ErrorBase([]DbufValSeq{Registry(Registry_data_value_not_accepted).ToDbufSeq()}, nil)
}
func DataKeyMissing() DbufError {
	return ErrorBase([]DbufValSeq{Registry(Registry_data_key_missing).ToDbufSeq()}, nil)
}
func TextError(text string) DbufError {
	return ErrorBase([]DbufValSeq{Text(text).ToDbufSeq()}, nil)
}
func ErrorWrap(outer DbufError, inner error) DbufError {
	wrapped := outer.Unwrap()
	wrapped = append(wrapped, inner)
	return ErrorBase(outer.DbufError(), wrapped)
}
func ErrorToValInternal(err error, internalFunc func(error) DbufValSeq) DbufValSeq {
	var wrapped DbufValSeq
	nest := false
	var dvs DbufValSeq
	if de, ok := err.(DbufError); ok {
		des := de.DbufError()
		if len(des) == 1 {
			dvs = des[0]
		} else {
			dvs = DbufValSeq{Val: Map(), Sequence: des}
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			uw := x.Unwrap()
			if uw != nil {
				wrapped = ErrorToValInternal(uw, internalFunc)
				nest = true
			}
		case interface{ Unwrap() []error }:
			wrapped = DbufValSeq{Val: Array()}
			for _, uw := range x.Unwrap() {
				wrapped.Sequence = append(wrapped.Sequence, ErrorToValInternal(uw, internalFunc))
				nest = true
			}
			if len(wrapped.Sequence) == 1 {
				wrapped = wrapped.Sequence[0]
			}
		}
	} else {
		if internalFunc != nil {
			dvs = internalFunc(err)
		} else {
			dvs = DbufValSeq{Val: Registry(Registry_error_internal)}
		}
	}
	if nest {
		if len(dvs.Sequence) == 0 {
			dvs.Sequence = []DbufValSeq{{Val: Registry(Registry_value)}, dvs}
			dvs.Val = Map()
		} else if len(dvs.Sequence) == 1 {
			dvs.Sequence = []DbufValSeq{{Val: Registry(Registry_value)}, dvs.Sequence[0]}
			dvs.Val = Map()
		}
		dvs.Sequence = append(dvs.Sequence, DbufValSeq{Val: Registry(Registry_error)})
		dvs.Sequence = append(dvs.Sequence, wrapped)
	}
	return dvs
}
func ErrorToVal(err error) DbufValSeq {
	return ErrorToValInternal(err, nil)
}