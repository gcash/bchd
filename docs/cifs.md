It is possible to configure bchd to store blockchain data on a CIFS mount.
Just point your bchd data directory at the mount, and make sure to turn
off asynchronous preemption.

```
export GODEBUG=asyncpreemptoff=1
bchd --datadir=/mnt
```

For more information about why async preemption needs to be disabled,
check out the following GitHub issue:
https://github.com/gcash/bchd/issues/367
