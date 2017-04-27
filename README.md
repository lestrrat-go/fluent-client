# go-fluent-client

[WIP] A fluentd client

# DESCRIPTION

```go
client, err := fluent.New()
if err != nil {
  ...
}

client.Post("tag", payload)


ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
if err := client.Shutdown(ctx); err != nil {
  ...
}
// TODO: implement Close() to forcefully close
```

