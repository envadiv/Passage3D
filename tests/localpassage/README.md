# LocalPassage

LocalPassage is a complete Passage testnet containerized with Docker and orchestrated with a simple docker-compose file. LocalPassage comes preconfigured with opinionated, sensible defaults for a standard testing environment.

LocalPassage comes with no initial state. In the future, we would like to be able to run LocalPassage with a mainnet export.

## Prerequisites

Ensure you have docker docker-compose, and golang installed:

```sh
# Docker
sudo apt-get remove docker docker-engine docker.io
sudo apt-get update
sudo apt install docker.io -y

# Docker compose
sudo apt install docker-compose -y

# Golang
curl -OL https://golang.org/dl/go1.19.1.linux-amd64.tar.gz
sudo tar -C /usr/local -xvf go1.19.1.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

Alternatively, internal teams can use the AWS Launch Template `passage-localnet-devbox-template` to provision an instance with these dependencies already installed.

## LocalPassage - No Initial State

The following commands must be executed from the root folder of the Passage repository.

1. Make any change to the passage code that you want to test

2. Initialize LocalPassage:

   ```bash
   make localnet-start
   ```

   The command:

   - Runs `passage testnet` to initialize the configuration files of the network. These will be stored under `./mytestnet`.
   - Runs the `docker-compose.yml` file to spin up the networked nodes in separate containers.

3. You can stop the chain using Ctrl + C.

4. When you are done you can clean up the environment with:

   ```bash
   make localnet-clean
   ```

   Which will remove the configuration files found under `./mytestnet`.
