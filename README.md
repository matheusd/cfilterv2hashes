# Decred cfilterv2 fetcher

This tool fetches all version 2 committed filters (cfilterv2) from a dcrd node
and writes (to stdout) a file containing every one of them.

The resulting file is meant to be used by dcrwallet to repair pre-dcp0005 cfilters
in a situation where they are determined as invalid.

At the end of the process, the chainhash (i.e. Blake256) of the full dataset
and a merkle root of the hashes of individual filters is presented so that
users and reviewers can verify the included dataset in the wallet is correct
from the PoV of their own local, trusted dcrd full node.

This data only needs to be generated for blocks prior to the activation of 
[DCP-0005](https://github.com/decred/dcps/blob/master/dcp-0005/dcp-0005.mediawiki)
given that blocks after its activation already include a commitment to the
corresponding cfilterv2 in their header.

| Network | Activation Height | Preconfigured File | Data Hash | Merkle Root |
| --- | --- | --- | --- | --- |
| *TestNet3* | 323328 | [testnet-data.bin](testnet-data.bin) | 619c08f5adda6f834212bbdaee3002fdc4efed731477af6c0fed490bbe2488d0 | e0580dabfae6732cbcadd9339cef550654ddb60ee561b8a13e064058b244c8ba |
| *MainNet* | 431488 | [mainnet-data.bin](mainnet-data.bin) |f95e09f9ded38f8d6c32e5158a1f286633881393659218c63f5ab0fc86b36c83 | a160c16cd56a0faefaed962fe7a06f7e1dd959832d292513fe681cf87bbe47aa |

Usage:

```shell
# TestNet
$ go run . -c localhost:19109 -u USER -P PASSWORD -t 323327 --testnet --progress -b > testnet-data.bin

# MainNet
$ go run . -c localhost:19109 -u USER -P PASSWORD -t 431488 --progress -b > mainnet-data.bin
```
