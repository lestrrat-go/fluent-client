# go-fluent-client

[WIP] A fluentd client

# DESCRIPTION

```go
client, err := fluent.New()
if err != nil {
  ...
}

client.Post("tag", payload)
```

