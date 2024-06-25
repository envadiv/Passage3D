# LocalPassage

LocalPassage is a complete Passage testnet containerized with Docker and orchestrated with a simple docker-compose file. LocalPassage comes preconfigured with opinionated, sensible defaults for a standard testing environment.

LocalPassage comes with no initial state. In the future, we would like to be able to run LocalPassage with a mainnet export.

## Prerequisites

Ensure you have docker and docker-compose installed:

```sh
# Docker
for pkg in docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc; do sudo apt-get remove $pkg; done
# Add Docker's official GPG key:
sudo apt-get update
sudo apt-get install ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# Add the repository to Apt sources:
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

If you're running docker as a non-root user you need to add your USER to the `docker` group to avoid appending every `docker` ccommand with `sudo`. Add `USER` to the `docker` group:

```bash
sudo groupadd docker
sudo usermod -aG docker $USER
newgrp docker
```

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

3. It runs the containers in deatched mode. To check the logs of the container nodes

   ```bash
   docker container logs <container-name>
   ```

   Where `container-name` can be `passagenode0`, `passagenode1`, `passagenode2` or `passagenode3`

4. When you are done you can clean up the environment with:

   ```bash
   make localnet-stop
   make localnet-clean
   ```

   Which will stop and remove the configuration files found under `./mytestnet`.
