// Copyright (c) 2014-2015 The btcsuite developers
// Copyright (c) 2015-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/crypto/blake256"
	"github.com/decred/dcrd/dcrutil/v3"
	"github.com/decred/dcrd/gcs/v2"
	"github.com/decred/dcrd/rpcclient/v5"
	"github.com/jessevdk/go-flags"
	"golang.org/x/sync/errgroup"
)

type config struct {
	RPCUser      string `short:"u" long:"rpcuser" description:"Username for RPC connections"`
	RPCPass      string `short:"P" long:"rpcpass" description:"Password for RPC connections"`
	RPCCert      string `long:"rpccert" description:"File containing the certificate file"`
	RPCConnect   string `short:"c" long:"rpcconnect" description:"Network address of dcrd RPC server"`
	TargetHeight uint64 `short:"t" long:"targetheight" description:"Target height to generate cfilters for. If empty, generates up to tip."`
	TestNet      bool   `long:"testnet" description:"Use the test network"`
	Progress     bool   `long:"progress" description:"Print progress in stderr"`
	Hashes       bool   `long:"hashes" description:"Generate hashes to stdout in text format"`
	Binary       bool   `short:"b" long:"binary" description:"Generate full cfilter data in binary format to stdout"`
}

type dcrdConfig struct {
	RPCUser string `short:"u" long:"rpcuser" description:"Username for RPC connections"`
	RPCPass string `short:"P" long:"rpcpass" description:"Password for RPC connections"`
	RPCCert string `long:"rpccert" description:"File containing the certificate file"`
	TestNet bool   `long:"testnet" description:"Use the test network"`
}

func main() {
	dcrdHomeDir := dcrutil.AppDataDir("dcrd", false)
	opts := &config{
		RPCCert: filepath.Join(dcrdHomeDir, "rpc.cert"),
	}
	dcrdCfgFile := filepath.Join(dcrdHomeDir, "dcrd.conf")
	if _, err := os.Stat(dcrdCfgFile); err == nil {
		// ~/.dcrd/dcrd.conf exists. Read precfg data from it.
		dcrdOpts := &dcrdConfig{}
		parser := flags.NewParser(dcrdOpts, flags.Default)
		err := flags.NewIniParser(parser).ParseFile(dcrdCfgFile)
		if err == nil {
			opts.RPCUser = dcrdOpts.RPCUser
			opts.RPCPass = dcrdOpts.RPCPass
			switch {
			case dcrdOpts.TestNet:
				opts.RPCConnect = "localhost:19109"
				opts.TestNet = true
			default:
				opts.RPCConnect = "localhost:9109"
			}
		}
	}

	parser := flags.NewParser(opts, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		var e *flags.Error
		if errors.As(err, &e) && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		parser.WriteHelp(os.Stderr)
		return
	}

	if opts.Hashes && opts.Binary {
		fmt.Println("Only specify one of --hashes and --binary")
		os.Exit(1)
	}

	// Connect to local dcrd RPC server using websockets.
	certs, err := ioutil.ReadFile(opts.RPCCert)
	if err != nil {
		log.Fatal(err)
	}
	connCfg := &rpcclient.ConnConfig{
		Host:         opts.RPCConnect,
		Endpoint:     "ws",
		User:         opts.RPCUser,
		Pass:         opts.RPCPass,
		Certificates: certs,
	}
	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}

	if blockCount < int64(opts.TargetHeight) {
		log.Fatalf("Cannot generate up to %d when block count is %d",
			opts.TargetHeight, blockCount)
	}
	if opts.TargetHeight == 0 {
		opts.TargetHeight = uint64(blockCount)
	}

	network := "mainnet"
	if opts.TestNet {
		network = "testnet3"
	}

	// Request cfilters in batches of 'step' blocks at a time since
	// fetching them is the slowest operation.
	step := int64(4000)
	cfilters := make([]*gcs.FilterV2, step)

	// Keep track of the hash of the full set of cfilter hashes so other
	// people can verify them easily.
	hasher := blake256.New()

	if opts.Progress {
		log.Printf("Generating up to block %d for %s", opts.TargetHeight, network)
	}
	target := int64(opts.TargetHeight)

	out := os.Stdout

	if opts.Hashes {
		out.Write([]byte("// Autogenerated by github.com/matheusd/cfiltersv2hashes\n\n"))
		out.Write([]byte("package validate\n\n"))
		out.Write([]byte(fmt.Sprintf("const cfilterv2RawHashes_%s string = \"", network)))

	}

	var totalCFilterSize, maxCFilterSize int64
	for h := int64(0); h <= target; h += step {

		// Fetch either a batch of size 'step' cfilters or only as many
		// as needed to reach the target height.
		if h+step > target {
			step = target - h + 1
			cfilters = cfilters[:step]
		}

		// Do so concurrently.
		var g errgroup.Group
		for i := int64(0); i < step; i++ {
			i := i
			g.Go(func() error {
				bh, err := client.GetBlockHash(h + i)
				if err != nil {
					return err
				}

				f, err := client.GetCFilterV2(bh)
				if err != nil {
					return err
				}
				cfilters[i] = f.Filter
				return nil
			})
		}
		err := g.Wait()
		if err != nil {
			log.Fatal(err)
		}

		// Now for every cfilter received, write out its hex
		// representation in the output string and keep track of the
		// stats.
		for i, cf := range cfilters {
			if opts.Hashes {
				fh := cf.Hash()
				out.Write([]byte(fmt.Sprintf("%s", fh.String())))
			}

			lenCf := int64(len(cf.Bytes()))
			if opts.Binary {
				// Write the length as a two byte, big endinan
				// uint16, then the cfilter data.
				var size [2]byte
				binary.BigEndian.PutUint16(size[:], uint16(lenCf))
				out.Write(size[:])
				out.Write(cf.Bytes())
			}

			totalCFilterSize += lenCf
			hasher.Write(cf.Bytes())

			if lenCf > maxCFilterSize {
				maxCFilterSize = lenCf
			}

			// Report on progress.
			if opts.Progress && (h+int64(i))%10000 == 0 {
				log.Printf("Generated up to height %d", h+int64(i))
			}
		}
	}

	if opts.Hashes {
		out.Write([]byte("\"\n"))
	}

	if opts.Progress {
		var cfsetHash chainhash.Hash
		cfsetHash.SetBytes(hasher.Sum(nil))
		log.Printf("Hash of raw data: %s\n", cfsetHash)
		log.Printf("Total CFilter size: %.2f MiB\n", float64(totalCFilterSize)/1024/1024)
		log.Printf("Avg CFilter size: %d bytes\n", totalCFilterSize/target)
		log.Printf("Max CFilter size: %d bytes\n", maxCFilterSize)

	}
	client.Shutdown()
}
