# Passage3D

## Getting Started
This document provides instructions for running a single node network on your local machine and then
submitting your first few transactions to that network using the command line. Running a single node network
is a great way to get familiar with Regen Ledger and its functionality.

## Prerequisites

### Git

For more information, see [Git](https://git-scm.com).

### Make

For more information, see [GNU Make](https://www.gnu.org/software/make/).

### Go

For more information, see [Go](https://golang.org/).

### Hardware

We recommend the following hardware specifications:

- 8GB RAM
- 4vCPUs
- 200GB Disk space

### Software

We recommend using Ubuntu 18.04 or 20.04.

The following instructions will install the necessary prerequisites on a Linux machine.

Install tools:

```bash
sudo apt install git build-essential wget jq -y
```

Download Go:

```bash
wget https://dl.google.com/go/go1.17.2.linux-amd64.tar.gz
```

Unpack Go download:

```bash
sudo tar -C /usr/local -xzf go1.17.2.linux-amd64.tar.gz
```

Set up environment:

```bash
echo '
export GOPATH=$HOME/go
export GOROOT=/usr/local/go
export GOBIN=$GOPATH/bin
export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> ~/.profile
```

Source profile file:

```bash
. ~/.profile
```

## Installation

### Install `Passage3d`

```shell
git clone https://github.com/envadiv/Passage3D
cd Passage3D
git checkout v1.0.0-rc1
make install

# verify the installation
passage3d version
```

### Create Accounts
```shell
passage3d keys add test
```