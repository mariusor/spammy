```sh
# generate bin file containing the various types of Object contents
go run aletheia.icu/broccoli -src ./data -var data -o data.gen.go
# run spammy on a FedBOX instance with authentication credentials
go run cmd/main.go --url https://fedbox.local --client aa52ae57-2ec2-4ddd-afcc-1fcbea6a29c0 --secret 'pa$$w0rd!'
```
