package protocol

const (
	Registry_reference         = 8
	Registry_operation         = 9
	Registry_authority_marker  = 10
	Registry_host              = 11
	Registry_header_store      = 12
	Registry_identity          = 135
	Registry_identity_key      = 136
	Registry_identity_recovery = 137
	Registry_deliver_message   = 138
	Registry_ed25519           = 139
	Registry_body_length       = 140
	Registry_stream_group      = 141
	Registry_header            = 142
	Registry_body              = 143
	Registry_footer            = 144
	Registry_not_authenticated = 145
	Registry_stream_id         = 146
	Registry_port              = 147
)

const (
	HeaderStoreCacheMaxSize = 1 << 16
	SmallRequestBufferSize  = 1 << 14
	ALPN                    = "dbuf/demo"
)
