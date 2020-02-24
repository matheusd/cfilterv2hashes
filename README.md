# Decred cfilterv2 hash fetcher

This tool fetches all version 2 committed filters (cfilterv2) from a dcrd node
and writes (to stdout) a file containing hashes for every one of them.

The resulting file is meant to be used by the dcrwallet code to validate whether
received cfilters from remote nodes are actually correct.

At the end of the process, the SHA-256 of the full dataset is presented so that
users and reviewers can verify the included dataset in the wallet is correct
from the PoV of their own local, trusted dcrd full node.

This data only needs to be generated for blocks prior to the activation of 
[DCP-0005](https://github.com/decred/dcps/blob/master/dcp-0005/dcp-0005.mediawiki)
given that blocks after its activation already include a commitment to the
corresponding cfilterv2 in their header.

| Network | Block Height |
| --- | --- |
| *TestNet3* | 323328 |
| *MainNet* | _(vote in progress)_ |

Usage:

```shell
$ go run . -c localhost:19109 -u USER -P PASSWORD -t 32337 --testnet --progress -b > testnet-data.bin
```
