# vCenter cluster testbed automation

## Overview

This directory contains scripts to automate VCSA/ESXi install and cluster configuration for developing and testing VIC.

## Dependencies

### govc

Install the latest release via https://github.com/vmware/govmomi/releases

### jq

Used here to derive static VCSA networking from its parent ESXi host.
But, you should already be using and loving jq for other tasks: http://stedolan.github.io/jq/

## Scripts

### create-esxi-vm.sh

This script creates a VM running stateless ESXi, booted via cdrom/iso.
It will create 2 disks:

* vSAN cache disk (Virtual SSD)

* vSAN store disk

The two vSAN disks will be unformatted, leaving them to be autoclaimed
by a vSAN enabled cluster.

Note that for a cluster to use vSAN, it will need at least 3 of these
ESXi VMs.

### create-vcsa-vm.sh

This script creates a VM with VCSA (Virtual Center Server Appliance)
installed.

### create-cluster.sh

The first argument to the script is the IP address of VCSA.
There must be at least three arguments that follow, IP addresses of
ESXi hosts, to form the cluster.

The script then creates the following managed objects:

* Datacenter (dc1)

* ClusterComputeResource (cluster1)

* DistributedVirtualSwitch (DSwitch)

* DistributedVirtualPortgroup (ExternalNetwork)

* DistributedVirtualPortgroup (InternalNetwork)

All of the given host systems are:

* Added to the ClusterComputeResource (cluster1)

* Added to the DistributedVirtualSwitch (DSwitch)

* Enabled for vSAN traffic (vmk0)

* Firewall configured to enable the remoteSerialPort rule

Cluster configuration includes:

* DRS enabled

* vSAN autoclaim of host system disks (results in shared Datastore "vsanDatastore")

## Example

This example will install VCSA, 3 ESXi VMs and create a cluster for
use by VIC.

```
export GOVC_URL="root:password@some-esx-host"

./create-vcsa-vm.sh -n "vic-${USER}-vcsa" $GOVC_URL

seq 1 3 | xargs printf "vic-${USER}-esxi-%03d\n" | xargs -P3 -n1 ./create-esxi-vm.sh $GOVC_URL

govc vm.ip -k "vic-${USER}-vcsa" "vic-${USER}-esxi-*" | xargs ./create-cluster.sh
```

## Licenses

Optional, if you want to assign licenses to VCSA or ESXi hosts.

Where "ESX_LICENSE" should be that of "vSphere 6 per CPU, Enterprise Plus".

And "VCSA_LICENSE" should be that of "vCenter Server 6, Standard".

```
ESX_LICENSE=... VCSA_LICENSE=... /assign-licenses.sh
```
