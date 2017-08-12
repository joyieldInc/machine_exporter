# machine_exporter
Simple server that scrapes current machine stats and
exports them via HTTP for Prometheus consumption

## Build

It is as simple as:

    $ make

## Running

    $ ./machine_exporter

With default options, machine_export will listen at 0.0.0.0:9009
To change default options, see:

    $ ./machine_exporter --help

## License

Copyright (C) 2017 Joyield, Inc. <joyield.com#gmail.com>

All rights reserved.

License under BSD 3-clause "New" or "Revised" License
