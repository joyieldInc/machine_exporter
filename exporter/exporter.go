/*
 * exporter - scrapes machine stats and exports for prometheus.
 * Copyright (C) 2017 Joyield, Inc. <joyield.com@gmail.com>
 * All rights reserved.
 */
package exporter

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/safchain/ethtool"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
)

const (
	namespace              = "machine"
	proc_stat              = "/proc/stat"
	proc_meminfo           = "/proc/meminfo"
	proc_net_dev           = "/proc/net/dev"
	used_memory            = "used_memory"
	available_memory       = "available_memory"
	total_memory           = "total_memory"
	used_cpu_user          = "used_cpu_user"
	used_cpu_sys           = "used_cpu_sys"
	used_cpu_iowait        = "used_cpu_iowait"
	used_cpu               = "used_cpu"
	used_cpu_idle          = "used_cpu_idle"
	cpu                    = "cpu"
	total_net_input_bytes  = "total_net_input_bytes"
	total_net_input_errs   = "total_net_input_errs"
	total_net_input_drop   = "total_net_input_drop"
	total_net_output_bytes = "total_net_output_bytes"
	total_net_output_errs  = "total_net_output_errs"
	total_net_output_drop  = "total_net_output_drop"
	interface_speed        = "interface_speed"
)

var (
	globalGauges = [][]string{
		{used_memory, "Current used memory"},
		{available_memory, "Current available memory"},
		{total_memory, "Current used memory"},
		{used_cpu_user, "Used cpu user"},
		{used_cpu_sys, "Used cpu sys"},
		{used_cpu_iowait, "Used cpu iowait"},
		{used_cpu, "Used cpu total"},
		{used_cpu_idle, "Used cpu idle"},
		{cpu, "Total cpu"},
	}
	netGauges = [][]string{
		{total_net_input_bytes, "Total net input bytes"},
		{total_net_input_errs, "Total net input errors"},
		{total_net_input_drop, "Total net input drop"},
		{total_net_output_bytes, "Total net output bytes"},
		{total_net_output_errs, "Total net output erros"},
		{total_net_output_drop, "Total net input drop"},
		{interface_speed, "Interface speed"},
	}
)

type Exporter struct {
	mutex        sync.RWMutex
	globalGauges map[string]prometheus.Gauge
	netGauges    map[string]*prometheus.GaugeVec
}

func NewExporter() (*Exporter, error) {
	e := &Exporter{
		globalGauges: map[string]prometheus.Gauge{},
		netGauges:    map[string]*prometheus.GaugeVec{},
	}
	for _, m := range globalGauges {
		e.globalGauges[m[0]] = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      m[0],
			Help:      m[1],
		})
	}
	for _, m := range netGauges {
		e.netGauges[m[0]] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      m[0],
			Help:      m[1],
		}, []string{"iface"})
	}
	return e, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, g := range e.globalGauges {
		ch <- g.Desc()
	}
	for _, g := range e.netGauges {
		g.Describe(ch)
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.resetMetrics()
	e.scrape()
	for _, g := range e.globalGauges {
		ch <- g
	}
	for _, g := range e.netGauges {
		g.Collect(ch)
	}
}

func (e *Exporter) resetMetrics() {
	for _, g := range e.netGauges {
		g.Reset()
	}
}

func unit(s string) int64 {
	v := strings.ToLower(s)
	if v == "kb" {
		return 1 << 10
	} else if v == "mb" {
		return 1 << 20
	} else if v == "gb" {
		return 1 << 30
	} else if v == "tb" {
		return 1 << 40
	}
	return 1
}

func (e *Exporter) scrape() {
	e.scrapeCpu()
	e.scrapeMem()
	e.scrapeNet()
}

func (e *Exporter) scrapeCpu() {
	s, err := ioutil.ReadFile(proc_stat)
	if err != nil {
		log.Printf("Read %s error:%v\n", proc_stat, err)
	} else {
		cpu_num := 1
		for _, line := range strings.Split(string(s), "\n") {
			ss := strings.Fields(line)
			if len(ss) < 6 {
				continue
			}
			if ss[0] == "cpu" {
				user, _ := strconv.ParseInt(ss[1], 10, 64)
				e.globalGauges[used_cpu_user].Set(float64(user) / 100.)
				sys, _ := strconv.ParseInt(ss[3], 10, 64)
				e.globalGauges[used_cpu_sys].Set(float64(sys) / 100.)
				idle, _ := strconv.ParseInt(ss[4], 10, 64)
				e.globalGauges[used_cpu_idle].Set(float64(idle) / 100.)
				iowait, _ := strconv.ParseInt(ss[5], 10, 64)
				e.globalGauges[used_cpu_iowait].Set(float64(iowait) / 100.)
				used := int64(0)
				for i := 1; i < len(ss); i++ {
					if i != 4 {
						v, _ := strconv.ParseInt(ss[i], 10, 64)
						used += v
					}
				}
				e.globalGauges[used_cpu].Set(float64(used) / 100.)
			} else if ss[0][:3] == "cpu" {
				id, _ := strconv.Atoi(ss[0][3:])
				if id+1 > cpu_num {
					cpu_num = id + 1
				}
			}
		}
		e.globalGauges[cpu].Set(float64(cpu_num))
	}
}

func (e *Exporter) scrapeMem() {
	s, err := ioutil.ReadFile(proc_meminfo)
	if err != nil {
		log.Printf("Read %s error:%v\n", proc_meminfo, err)
	} else {
		mem := map[string]int64{
			"MemTotal:":     0,
			"MemFree:":      0,
			"MemAvailable:": 0,
			"Buffers:":      0,
			"Cached:":       0}
		for _, line := range strings.Split(string(s), "\n") {
			ss := strings.Fields(line)
			if len(ss) < 3 {
				continue
			}
			if _, ok := mem[ss[0]]; ok {
				v, _ := strconv.ParseInt(ss[1], 10, 64)
				v *= unit(ss[2])
				mem[ss[0]] = v
			}
		}
		total := mem["MemTotal:"]
		available := mem["MemAvailable:"]
		if available == 0 {
			available = mem["MemFree:"] + mem["Buffers:"] + mem["Cached:"]
		}
		e.globalGauges[used_memory].Set(float64(total - available))
		e.globalGauges[available_memory].Set(float64(available))
		e.globalGauges[total_memory].Set(float64(total))
	}
}

func (e *Exporter) scrapeNet() {
	s, err := ioutil.ReadFile(proc_net_dev)
	if err != nil {
		log.Printf("Read %s error:%v\n", proc_net_dev, err)
		return
	}
	lines := strings.Split(string(s), "\n")
	if len(lines) <= 2 {
		return
	}
	lines = lines[2:]
	ifaces := []string{}
	for _, line := range lines {
		segs := strings.Split(line, ":")
		if len(segs) != 2 {
			continue
		}
		iface := strings.Trim(segs[0], " ")
		ss := strings.Fields(segs[1])
		if len(ss) < 16 {
			continue
		}
		ifaces = append(ifaces, iface)
		e.setNet(total_net_input_bytes, ss[0], iface)
		e.setNet(total_net_input_errs, ss[2], iface)
		e.setNet(total_net_input_drop, ss[3], iface)
		e.setNet(total_net_output_bytes, ss[8], iface)
		e.setNet(total_net_output_errs, ss[10], iface)
		e.setNet(total_net_output_drop, ss[11], iface)
	}
	if len(ifaces) > 0 {
		et, err := ethtool.NewEthtool()
		if err != nil {
			log.Printf("New ethtool err:%v\n", err)
			return
		}
		defer et.Close()
		cmd := &ethtool.EthtoolCmd{}
		for _, iface := range ifaces {
			if iface == "lo" {
				e.netGauges[interface_speed].WithLabelValues(iface).Set(0)
				continue
			}
			speed, err := et.CmdGet(cmd, iface)
			if err != nil {
				log.Printf("ethtool cmdget %s err:%v\n", iface, err)
			} else {
				e.netGauges[interface_speed].WithLabelValues(iface).Set(float64(speed << 17))
			}
		}
	}
}

func (e *Exporter) setNet(name, val, iface string) {
	v, err := strconv.ParseInt(val, 10, 64)
	if err == nil {
		e.netGauges[name].WithLabelValues(iface).Set(float64(v))
	}
}
