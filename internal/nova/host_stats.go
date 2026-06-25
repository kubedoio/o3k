package nova

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type hostResourceStats struct {
	vcpus      int
	memoryMB   int
	freeMB     int
	localGB    int
	freeGB     int
	runningVMs int
	hostname   string
}

func readHostStats(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) hostResourceStats {
	s := hostResourceStats{vcpus: 1, memoryMB: 1024, freeMB: 512, localGB: 100, freeGB: 50}

	s.hostname, _ = os.Hostname()
	if s.hostname == "" {
		s.hostname = "o3k-node-1"
	}

	// CPU count from /proc/cpuinfo (one "processor" line per logical CPU)
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		scanner := bufio.NewScanner(f)
		cpus := 0
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "processor") {
				cpus++
			}
		}
		f.Close()
		if cpus > 0 {
			s.vcpus = cpus
		}
	}

	// Memory from /proc/meminfo
	if f, err := os.Open("/proc/meminfo"); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}
			kb, _ := strconv.Atoi(fields[1])
			switch fields[0] {
			case "MemTotal:":
				s.memoryMB = kb / 1024
			case "MemAvailable:":
				s.freeMB = kb / 1024
			}
		}
		f.Close()
	}

	// Disk space via syscall.Statfs on the data directory
	dataDir := os.Getenv("O3K_DATA_DIR")
	if dataDir == "" {
		dataDir = "/var/lib/o3k"
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err == nil {
		blockSize := int64(stat.Bsize)
		s.localGB = int(int64(stat.Blocks) * blockSize / (1024 * 1024 * 1024))
		s.freeGB = int(int64(stat.Bavail) * blockSize / (1024 * 1024 * 1024))
		if s.localGB < 1 {
			s.localGB = 1
		}
		if s.freeGB < 0 {
			s.freeGB = 0
		}
	}

	// Running VMs from DB (power_state=1 means running)
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM instances WHERE power_state = 1").Scan(&s.runningVMs)

	return s
}

func buildHypervisorJSON(s hostResourceStats, includeCPUInfo bool) map[string]any {
	h := map[string]any{
		"id":                  1,
		"hypervisor_hostname": s.hostname,
		"state":               "up",
		"status":              "enabled",
		"hypervisor_type":     "QEMU",
		"hypervisor_version":  2012000,
		"vcpus":               s.vcpus,
		"memory_mb":           s.memoryMB,
		"local_gb":            s.localGB,
		"vcpus_used":          0,
		"memory_mb_used":      s.memoryMB - s.freeMB,
		"local_gb_used":       s.localGB - s.freeGB,
		"free_disk_gb":        s.freeGB,
		"free_ram_mb":         s.freeMB,
		"running_vms":         s.runningVMs,
	}
	if includeCPUInfo {
		h["cpu_info"] = fmt.Sprintf(
			`{"arch":"x86_64","model":"host","vendor":"unknown","features":[],"topology":{"cores":%d,"threads":1,"sockets":1}}`,
			s.vcpus,
		)
	}
	return h
}
