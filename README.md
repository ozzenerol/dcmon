# dcmon

Agentless fleet monitor over SSH written in Go.

This is one of my first Go projects, I have around 3 days of experience as I'm writing this README, so have mercy.

I'm using it to have a quick overview of all my physical and virtual machines plus my linux containers.

```sh
cp config.json.example config.json   # add your hosts
go run .
```

Commands: `status [hostname]`, `exit`.
