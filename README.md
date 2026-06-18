# dbuf-demo-go

A proof of concept implementation of the [DBUF protocol](https://github.com/bintoca/dbuf).

### Run the server
```
cd demo-server
go run .
```
The server will create a badgerDB to store identities and listen on UDP port 443

### Run the client
```
cd demo-client
go run .
```
The client will prompt for a message to send to the server. After entering the message the client will:
- Create a new Ed25519 key pair for authentication
- Save the key pair to a json file
- Create a new identity on the server with the public key
- Authenticate the identity with the server with a signature
- Send the message to the server

Subsequent runs of the client will reuse the same identity/key.
