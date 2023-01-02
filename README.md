# PassiveReplicationTempl

## Set-up
Run the following in separate terminals:

- `go run rmserver.go 5000` 
- `go run rmserver.go 5001` 
- `go run rmserver.go 5002` 
- `go run feserver.go 4000` 
- `go run feserver.go 4001` 
- `go run client.go 4000` 
- `go run client.go 4001`