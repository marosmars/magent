# VPP MONITORING AGENT

This is a lightweight monitoring agent for VPP. It runs as a dedicated process, communicating with VPP using its
binary shared memory APIs.

It has very small footprint and does not affect other agents running on top of VPP.

## Build

To build the agent:

### Install and set up golang:

    https://golang.org/doc/install#install

Note: Make sure to set GOPATH variable

    mkdir -p $GOPATH/src
    mkdir -p $GOPATH/bin
    mkdir -p $GOPATH/pkg

### Get the project:
    
    mkdir -p $GOPATH/src/pnda/vpp/
    cd $GOPATH/src/pnda/vpp
    git clone ... monitoring   
    cd monitoring
    
### Install VPP

Add required repository:

    https://wiki.fd.io/view/VPP/Installing_VPP_binaries_from_packages
    
Note: Version 17.01 of VPP is supported by this agent.

    sudo apt-get install vpp vpp-dev vpp-dpdk-dkms
    
Note: Usually you install just vpp and vpp-dpdk-dkms for VPP runtime, but for the build of this agent, vpp-dev is necessary as well.

### Build the project

    make BUILD_NUMBER=99 full-build deb-xenial-package
    
Note: Each supported OS has a dedicated *package target, the full-build target builds a binary in GOPATH/bin target
Note: The produced package will be in $GOPATH/bin and can be installed using `dpkg -i` command on ubuntu
Note: Build number is used to produce build version in the deb packages

## Run

To run the agent:

### Run VPP and the agent:

First make sure VPP is running:

    sudo service vpp start
    
You can start monitoring agent using a binary:

    sudo $GOPATH/bin/monitoring -debug -wiring-file=$GOPATH/src/pnda/vpp/monitoring/configuration.yaml -vpp-uuid=`hostid`
    
Or you can use a service if you installed the package:

    sudo service vpp-monitoring-agent start
