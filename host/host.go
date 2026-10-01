// Package host that defines the host model and its operations
package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/tabwriter"

	"golang.org/x/crypto/ssh"
)

type HostType string

const (
	Proxmox								HostType	= "PROXMOX"
	VirtualMachine				HostType	= "VM"
	LinuxVirtualMachine		HostType	= "LXC"
	NetworkAreaStorage		HostType	= "NAS"

	ExpectedFileFormat		string		= ".json"

	RowFormat							string		= "%-16s %-8s %-6s %-13s %-13s %s\n"

	SystemCommandUptime		string		= `awk '{d=int($1/86400); h=int($1/3600); if (d>=1) printf "%dD", d; else printf "%dH", (h<1 ? 1 : h)}' /proc/uptime`
	SystemCommandCPU			string = `{ head -1 /proc/stat; sleep 1; head -1 /proc/stat; } | awk '{b=$2+$3+$4+$7+$8+$9; t=b+$5+$6} NR==1{b1=b; t1=t} NR==2{printf "%d%%", (b-b1)*100/(t-t1)}'`
	SystemCommandMemory		string = `awk '/MemTotal/{t=$2} /MemAvailable/{a=$2} END{printf "%.1fG/%.1fG", (t-a)/1048576, t/1048576}' /proc/meminfo`
	SystemCommandGPU			string = `nvidia-smi --query-gpu=memory.used,memory.total --format=csv,noheader,nounits | awk -F', ' 'NR==1{printf "%.1fG/%.1fG", $1/1024, $2/1024}'`
  SystemCommandDocker		string = `docker ps -a --format '{{.State}}' | awk '$1=="running"{r++} END{printf "%d/%d", r, NR}'`
)

type HostFlags struct {
	NvidiaGpu	bool
	Docker		bool
}

type Host struct {
	Hostname	string
	Type			string
	User			string
	IP				string
	Password	string
	Exclude		bool
	Flags			HostFlags
}

type HostTable struct {
	Hostname		string
	Uptime			string
	Memory			string
	CPU					string
	GPU					string
	Containers	string
}

func Load(filename string) ([]Host, error) {
	var hosts []Host
	
	if filename == "" {
		return hosts, fmt.Errorf("`filename` must not be empty")
	}

	if !strings.HasSuffix(filename, ExpectedFileFormat) {
		return hosts, fmt.Errorf("`filename` must have %s file format", ExpectedFileFormat)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return hosts, fmt.Errorf("could not read %s: %w", filename, err)	
	}

	if err := json.Unmarshal(data, &hosts); err != nil {
		return hosts, fmt.Errorf("could not parse %s: %w", filename, err)
	}

	return hosts, nil
}

func Status(hosts []Host, args []string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("`hosts` cannot be an empty array")
	}

	if len(args) > 0 {
		hosts = filterByType(hosts, args[0])
	}

	rows := make([]HostTable, len(hosts))
	errs := make([]error, len(hosts))

	printHeader()

	var wg sync.WaitGroup
	for i, h := range hosts {
		if h.Exclude {
			continue
		}
		
		wg.Go(func() {
			rows[i], errs[i] = collect(h)
			printRow(rows[i])
		})
		
	}	
	wg.Wait()
	

	return errors.Join(errs...)
}

func StatusByHostname(hosts []Host, hostname string) error {
	if hostname == "" {
		return fmt.Errorf("`hostname` cannot be empty")
	}

	var host Host
	var found bool

	for i, h := range hosts {
		if h.Hostname == hosts[i].Hostname {
			host = hosts[i]
			found = true
		}
	}

	if !found {
		return fmt.Errorf("%s does not exist", hostname)
	}

	printHeader()
	
	rowResult, err := collect(host)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	printRow(rowResult)

	return nil
}

func filterByType(hosts []Host, hostType string) ([]Host){
	var retHosts []Host
	for _, h := range hosts {
		if h.Type == hostType {
			retHosts = append(retHosts, h)
		} 
	}	
	return retHosts
}

func collect(h Host) (HostTable, error) {
	client, err := connect(h)
	if err != nil {
		return HostTable{}, fmt.Errorf("could not connect to (%s) %s: %w", h.IP, h.Hostname, err)
	}
	defer client.Close()

	uptime, err := readUptime(client)
		if err != nil {
			return HostTable{}, fmt.Errorf("could not get uptime from (%s) %s: %w", h.IP, h.Hostname, err)
		}

		cpu, err := readCPU(client)
		if err != nil {
			return HostTable{}, fmt.Errorf("could not get cpu stats from (%s) %s: %w", h.IP, h.Hostname, err)
		}

		mem, err := readMem(client)
		if err != nil {
			return HostTable{}, fmt.Errorf("could not get mem stats from (%s) %s: %w", h.IP, h.Hostname, err)
		}

		var gpu string
		if h.Flags.NvidiaGpu {
			gpu, err = readGPU(client)
			if err != nil {
				return HostTable{}, fmt.Errorf("could not get gpu stats from (%s) %s: %w", h.IP, h.Hostname, err)
			}
		}

		var docker string
		if h.Flags.Docker {
			docker, err = readDocker(client)
			if err != nil {
				return HostTable{}, fmt.Errorf("could not get docker stats from (%s) %s: %w", h.IP, h.Hostname, err)
			}
		}
	
		return HostTable{Hostname: h.Hostname, Uptime: uptime, CPU: cpu, Memory: mem, GPU: gpu, Containers: docker}, nil
}

func connect(host Host) (*ssh.Client, error) {
	config := &ssh.ClientConfig {
		User:							host.User,
		Auth:							[]ssh.AuthMethod { ssh.Password(host.Password) },
		HostKeyCallback:	ssh.InsecureIgnoreHostKey(),
	}

	return ssh.Dial("tcp", host.IP + ":22", config)
}

func readUptime(client *ssh.Client) (string, error) {
	return run(client, SystemCommandUptime)
}

func readCPU(client *ssh.Client) (string, error) {
	return run(client, SystemCommandCPU)
}

func readMem(client *ssh.Client) (string, error) {
	return run(client, SystemCommandMemory)
}

func readGPU(client *ssh.Client) (string, error) {
	return run(client, SystemCommandGPU)
}

func readDocker(client *ssh.Client) (string, error) {
	return run(client, SystemCommandDocker)
}

func run(client *ssh.Client, cmd string) (string, error) {
  session, err := client.NewSession()
  if err != nil {
    return "", err
  }
  defer session.Close()

  out, err := session.CombinedOutput(cmd)
  return string(out), err
}

func printHeader() {
  fmt.Printf(RowFormat, "HOSTNAME", "UPTIME", "CPU", "MEM", "GPU", "CONTAINERS")
}

func printRow(row HostTable) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	gpu := row.GPU
	if gpu == "" {
		gpu = "-"
	}
	containers := row.Containers
	if containers == "" {
		containers = "-"
	}
	fmt.Fprintf(w, RowFormat, row.Hostname, row.Uptime, row.CPU, row.Memory, gpu, containers)
	w.Flush()	
}

func print(rows []HostTable) {
  w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0) 
  for _, r := range rows {
    gpu := r.GPU
    if gpu == "" {
      gpu = "-"
    }
		containers := r.Containers
		if containers == "" {
			containers = "-"
		}
    fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Hostname, r.Uptime, r.CPU, r.Memory, gpu, containers)
  }
  w.Flush()
}
