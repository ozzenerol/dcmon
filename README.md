# dcmon

Agentless fleet monitor over SSH written in Go.

This is one of my first Go projects, I have around 3 days of experience as I'm writing this README, so have mercy.

I'm using it to have a quick overview of all my physical and virtual machines plus my linux containers.

```sh
cp config.json.example config.json   # add your hosts
go run .
```

Commands: `status [host_type]`, `exit`.

Available host types:
```
PROXMOX
VM
LXC
NAS
```

<img width="920" height="559" alt="image" src="https://github.com/user-attachments/assets/9ce1afde-9faf-4429-90cf-bb07bff749bd" />
