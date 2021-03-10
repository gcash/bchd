# BCHD regtest harness

This setup allows for additional tests into bchd's continuous integration pipeline using regtest.  Docker compose is used to setup a regtest network with two bchd instances connected.

## Travis CI

These tests are configured to run in `.travis.yml` with the docker service added along with the script `sh -c 'cd ./bchrpc/regtest && ./test.sh'`.

## Run Tests

Run the tests against the regtest network using:

```
$ ./test.sh
```

## Debugging with VSCode

An example `launch.json` file has been included for debugging with vscode.  Copy this config into the `.vscode/launch.json` file created by VSCode.
