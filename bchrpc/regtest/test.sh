#!/bin/bash

echo "INFO: Cleaning up from previous runs..."
docker-compose down
docker rmi bchd_regtest nodejs_regtest
rm -f ./rpc.bchd1.*

echo "INFO: Creating bchd regtest network from source"
docker-compose up -d

echo "INFO: Running mocha tests in docker"
docker-compose exec nodejs ./_test.sh
exit_code=$?

echo "INFO: Cleaning up."
docker-compose down
rm -f ./rpc.bchd1.*

if [ $exit_code -eq 0 ]; then
  echo "INFO: All regtest network tests pass (code: $exit_code)"
else
  echo "ERROR: One or more regtest network tests failed (code: $exit_code)"
fi

exit $exit_code
