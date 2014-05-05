// Copyright (c) 2014 Square, Inc
// +build linux

package osmain

import (
	"fmt"
	"github.com/square/prodeng/inspect/cpustat"
	"github.com/square/prodeng/inspect/diskstat"
	"github.com/square/prodeng/inspect/interfacestat"
	"github.com/square/prodeng/inspect/memstat"
	"github.com/square/prodeng/inspect/misc"
	"github.com/square/prodeng/inspect/pidstat"
	"github.com/square/prodeng/metrics"
	"path/filepath"
)

type LinuxStats struct {
	dstat  *diskstat.DiskStat
	ifstat *interfacestat.InterfaceStat
	cg_mem *memstat.CgroupStat
	cg_cpu *cpustat.CgroupStat
	procs  *pidstat.ProcessStat
	cstat  *cpustat.CPUStat
}

func RegisterOsDependent(m *metrics.MetricContext, d *OsIndependentStats) *LinuxStats {

	s := new(LinuxStats)
	s.dstat = diskstat.New(m)
	s.ifstat = interfacestat.New(m)
	s.procs = d.Procs // grab it because we need to for per cgroup cpu usage
	s.cstat = d.Cstat
	s.cg_mem = memstat.NewCgroupStat(m)
	s.cg_cpu = cpustat.NewCgroupStat(m)

	return s
}

func PrintOsDependent(s *LinuxStats) {

	type cg_stat struct {
		cpu *cpustat.PerCgroupStat
		mem *memstat.PerCgroupStat
	}

	fmt.Println("---")
	for d, o := range s.dstat.Disks {
		fmt.Printf("disk: %s usage: %3.1f%%\n", d, o.Usage())
	}

	fmt.Println("---")
	for iface, o := range s.ifstat.Interfaces {
		fmt.Printf("iface: %s TX: %s/s, RX: %s/s\n", iface,
			misc.BitSize(o.TXBandwidth()),
			misc.BitSize(o.RXBandwidth()))
	}

	fmt.Println("---")
	// so much for printing cpu/mem stats for cgroup together
	cg_stats := make(map[string]*cg_stat)
	for name, mem := range s.cg_mem.Cgroups {
		name, _ = filepath.Rel(s.cg_mem.Mountpoint, name)
		_, ok := cg_stats[name]
		if !ok {
			cg_stats[name] = new(cg_stat)
		}
		cg_stats[name].mem = mem
	}

	for name, cpu := range s.cg_cpu.Cgroups {
		name, _ = filepath.Rel(s.cg_cpu.Mountpoint, name)
		_, ok := cg_stats[name]
		if !ok {
			cg_stats[name] = new(cg_stat)
		}
		cg_stats[name].cpu = cpu
	}

	for name, v := range cg_stats {
		var out string

		out = fmt.Sprintf("cgroup:%s ", name)
		if v.cpu != nil {
			// get CPU usage per cgroup from pidstat
			// unfortunately this is not exposed at cgroup level
			cpu_usage := s.procs.CPUUsagePerCgroup(name)
			out += fmt.Sprintf("cpu: %3.1f%% ", cpu_usage)
			out += fmt.Sprintf(
				"cpu_throttling: %3.1f%% (%.1f/%d) ",
				v.cpu.Throttle(), v.cpu.Quota(),
				(len(s.cstat.CPUS()) - 1))
		}

		if v.mem != nil {
			out += fmt.Sprintf(
				"mem: %3.1f%% (%s/%s) ",
				(v.mem.Usage()/v.mem.SoftLimit())*100,
				misc.ByteSize(v.mem.Usage()), misc.ByteSize(v.mem.SoftLimit()))
		}
		fmt.Println(out)
	}

}