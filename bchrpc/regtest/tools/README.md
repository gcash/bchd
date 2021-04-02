# Regtest tools

Methods and commands that allow additional observation while debugging node behavior can be placed here. These tools are intended to be run on an ad-hoc basis separate from the tests located in the test folder.

## Interaction with bchd3
The docker-compose regtest network with 2 connected nodes can be started using `$ docker-compose up -d` from the regtest directory.  A third bchd container `bchd3` is created but doesn't have any node started. This `bchd3` container is intended to be interacted with directly using `$ docker-compose exec bchd3 bash` so you can start and stop the node as needed for debugging and observational purposes.

### Example commands for bchd3 interaction

* Start node 3 with no indexes: `bchd --regtest --rpcuser=bitcoin --rpcpass=password --notls --addpeer=bchd1 --regtestanyhost` 
* Start node 3 with all indexes: `bchd --regtest --rpcuser=bitcoin --rpcpass=password --notls --addpeer=bchd1 --regtestanyhost --regtestnoreset --txindex --slpindex --addrindex`
* Get node 3 peer info: `bchctl -u=bitcoin -P=password --rpcserver=localhost:18334 --notls getpeerinfo`

**Note:** The flag `--regtestanyhost` allows node 3 to connect to the other peers on the docker network.  Without this the node won't sync to non-localhost peers in the docker-compose network. The flag `--regtestnoreset` prevents deletion of regtest blockchain data from previous runs.  By default, regtest data dir is reset at node startup.

## ./tools/Generator.ts 

This script keeps `bchd2` node actively generating one block per second for 1000 blocks.

Example usage:

1. Make sure the regtest network is running via `$ docker-compose up -d` (must cd into the regtest directory)
2. Start the generator script via `$ npx ts-node ./tools/generator.ts`.  This will run until all 1000 blocks are generated.
3. Shell into the `bchd3` container and start, stop, or inspect the node as required for observation.
