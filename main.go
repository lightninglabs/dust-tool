package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc/verrpc"
)

var (
	defaultLndDir          = btcutil.AppDataDir("lnd", false)
	defaultTLSCertFilename = "tls.cert"
	defaultTLSCertPath     = filepath.Join(
		defaultLndDir, defaultTLSCertFilename,
	)

	defaultDataDir     = "data"
	defaultChainSubDir = "chain"

	defaultMacaroonPath = filepath.Join(
		defaultLndDir, defaultDataDir, defaultChainSubDir, "bitcoin",
		"mainnet", "admin.macaroon",
	)

	defaultNet = "mainnet"

	defaultDustThreshold = uint64(500000)

	host = flag.String("host", "localhost:10009", "host of the target lnd node")

	tlsPath = flag.String("tlspath", defaultTLSCertPath, "path to the TLS cert of the target lnd node")

	macaroonPath = flag.String("macdir", defaultMacaroonPath, "path of admin.macaroon for the target lnd node")

	network = flag.String("network", defaultNet, "the network the lnd node is running on")

	checkChans = flag.Bool("check-chans", false, "whether to check existing channels for dust_limit_satoshis exposure")

	dustThreshold = flag.Uint64("dustexposure", defaultDustThreshold, "sets the dust threshold in satoshis for channel close recommendations - must be specified with checkchans")

	acceptorTimeout = 10 * time.Second

	minimumBuildTags = []string{}

	minimumCompatibleVersion = &verrpc.Version{
		AppMajor:  0,
		AppMinor:  0,
		AppPatch:  0,
		BuildTags: minimumBuildTags,
	}
)

func main() {
	flag.Parse()

	// First check whether or not we just want to check the channels.
	if *checkChans {
		// Check existing channels for dust exposure and output
		// recommendations.
		checkDustChannels()
		return
	}

	// Else, we'll start intercepting dusty OpenChannel requests.
	dustAcceptor()
}

// checkDustChannels calls ListChannels and recommends which channels should be
// closed due to potential dust exposure. This only evaluates confirmed
// channels since ListChannels does not return pending channels.
func checkDustChannels() {
	// Create new gRPC client.
	client, err := createRpcClient()
	if err != nil {
		log.Fatalf("unable to create gRPC client: %v", err)
		return
	}

	// Create a calling context.
	mainCtx := context.Background()

	// Call ListChannels to get the channels we should evaluate.
	channelResp, err := client.Client.ListChannels(mainCtx, false, false)
	if err != nil {
		log.Fatalf("call to listchannels failed: %v", err)
		return
	}

	log.Print("Evaluating set of channels for dust exposure")

	for _, channel := range channelResp {
		evaluateChannelDust(channel)
	}
}

// evaluateChannelDust takes in lndclient.ChannelInfo and outputs whether or
// not the underlying channel should be closed. It implements a simple
// heuristic to determine dust exposure and compares this against the defined
// dust threshold.
func evaluateChannelDust(info lndclient.ChannelInfo) {
	// max_dust_limit * (max_accepted_htlcs_A + max_accepted_htlcs_B)
	localConstraints := info.LocalConstraints
	remoteConstraints := info.RemoteConstraints

	localDustLimit := localConstraints.DustLimit
	remoteDustLimit := remoteConstraints.DustLimit
	localMaxAccepted := localConstraints.MaxAcceptedHtlcs
	remoteMaxAccepted := remoteConstraints.MaxAcceptedHtlcs

	// Determine the higher dust limit.
	maxDustLimit := localDustLimit
	if maxDustLimit < remoteDustLimit {
		maxDustLimit = remoteDustLimit
	}

	totalMaxHTLCs := localMaxAccepted + remoteMaxAccepted

	maxDustExposure := maxDustLimit * btcutil.Amount(totalMaxHTLCs)

	chanpoint := info.ChannelPoint

	if maxDustExposure > btcutil.Amount(*dustThreshold) {
		log.Printf("Consider closing chanpoint(%v), dust exposure(%v)"+
			" higher than threshold(%v)", chanpoint,
			maxDustExposure, btcutil.Amount(*dustThreshold))
	}

	// - Should we try to compare against max_pending_amt_msat (sum)?
}

// dustAcceptor implements the logic to intercept dusty OpenChannel requests.
func dustAcceptor() {
	// Create new gRPC client.
	client, err := createRpcClient()
	if err != nil {
		log.Fatalf("unable to create gRPC client: %v", err)
		return
	}

	// Create a calling context.
	mainCtx := context.Background()

	log.Print("Creating dust acceptor, no output will be recorded unless " +
		"it errors out.")

	// Call the ChannelAcceptor.
	errChan, err := client.Client.ChannelAcceptor(
		mainCtx, acceptorTimeout, channelPredicate,
	)
	if err != nil {
		log.Fatalf("unable to setup ChannelAcceptor: %v", err)
		return
	}

	<-errChan
}

// createRpcClient attempts to create a gRPC client with the provided
// parameters and returns an error if it failed.
func createRpcClient() (*lndclient.GrpcLndServices, error) {
	client, err := lndclient.NewLndServices(
		&lndclient.LndServicesConfig{
			LndAddress:         *host,
			Network:            lndclient.Network(*network),
			CustomMacaroonPath: *macaroonPath,
			TLSPath:            *tlsPath,
			CheckVersion:       minimumCompatibleVersion,
		},
	)
	return client, err
}

// channelPredicate is the ChannelAcceptor predicate that implements a dust
// rejection heuristic.
func channelPredicate(ctx context.Context,
	req *lndclient.AcceptorRequest) (*lndclient.AcceptorResponse, error) {

	// Define the minimum dust limit we'll accept. This will be the dust
	// limit of the smallest P2WSH output considered standard.
	minDustLimit := btcutil.Amount(330)

	// Define the maximum dust limit we'll accept. This will be:
	// 3 * minDustLimit.
	maxDustLimit := minDustLimit * 3

	resp := &lndclient.AcceptorResponse{}

	// Compare the received dust limit from OpenChannel against our bounds.
	receivedLimit := req.DustLimit
	if receivedLimit < minDustLimit || receivedLimit > maxDustLimit {
		// Reject the channel.
		resp.Accept = false
		return resp, nil
	}

	// Otherwise, the limit was within the acceptable bounds, so we'll
	// accept the channel.
	resp.Accept = true
	return resp, nil
}
