/*
 * machine_exporter - scrapes machine stats and exports for prometheus.
 * Copyright (C) 2017 Joyield, Inc. <joyield.com@gmail.com>
 * All rights reserved.
 */
package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"machine_exporter/exporter"
	"net/http"
)

func main() {
	var (
		bind = flag.String("bind", ":9009", "Listen address")
	)
	flag.Parse()
	exporter, err := exporter.NewExporter()
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*bind, nil))
}
