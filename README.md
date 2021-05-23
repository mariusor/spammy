```sh
# generate bin file containing the various types of Object contents
go run aletheia.icu/broccoli -src ./data -var data -o data.gen.go
# run spammer on fedbox instance
go run cmd/main.go 
```
