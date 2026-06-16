package basic

const (
	Registry_nonexistent             = 0
	Registry_describe_no_value       = 1
	Registry_value                   = 4
	Registry_error                   = 65
	Registry_error_internal          = 69
	Registry_incomplete_stream       = 70
	Registry_data_error              = 128
	Registry_data_type_not_accepted  = 129
	Registry_data_value_not_accepted = 130
	Registry_data_key_not_accepted   = 131
	Registry_data_key_missing        = 132
	Registry_end_marker              = 133
	Registry_data_path               = 134
	Registry_magic_number            = 14606046
)

const (
	Parse_type_integer    = 0
	Parse_type_float      = 1
	Parse_type_text       = 2
	Parse_type_bytes      = 3
	Parse_type_array      = 4
	Parse_type_map        = 5
	Parse_type_registry   = 6
	Parse_type_uint       = 7
	Single_byte_threshold = 24
)

const maxU32 = 0xFFFFFFFF
