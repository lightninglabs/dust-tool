# Dust Exposure Tool

## Building
- Requires Go 1.16
- Run the following command:
```
$ go build
```

## DustAcceptor mode
- The dust tool can intercept malicious `open_channel` requests with high `dust_limit_satoshis` values.
- Simply run it like so:
```
./dust-tool -network=<mainnet/testnet/etc> -macpath=<path-of-macaroon> -host=<host:port of RPC server>
```
- You should see output like this:
```
2021/09/30 18:06:47 Creating dust acceptor, no output will be recorded unless it errors out.
```

## Channel Close recommendations
- The dust tool can also give channel close recommendations based on a confirmed channel's `dust_limit_satoshis` and `max_accepted_htlcs` values.
- Simply run it like so, optionally specifying `-dustexposure` in satoshis:
```
./dust-tool -network=<mainnet/testnet/etc> -macpath=<path-of-macaroon> -host=<host:port of RPC server> -check-chans -dustexposure=1000000
```
- If a potentially risky channel is found, you should receive output like this:
```
2021/09/30 18:12:59 Evaluating set of channels for dust exposure
2021/09/30 18:12:59 Consider closing chanpoint(e2b43845a099c7ab98345a4604b24d7fbb4c19cec97911a9c1c250b57c762220:0), dust exposure(0.0483 BTC) higher than threshold(0.0049 BTC)
```
- If no output is returned, you are safe for the dust threshold specified!

## Help
```
Usage of ./dust-tool:
  -check-chans
        whether to check existing channels for dust_limit_satoshis exposure
  -dustexposure uint
        sets the dust threshold in satoshis for channel close recommendations - must be specified with checkchans (default 500000)
  -host string
        host of the target lnd node (default "localhost:10009")
  -macpath string
        path of admin.macaroon for the target lnd node (default "/path/admin.macaroon")
  -network string
        the network the lnd node is running on (default "mainnet")
  -tlspath string
        path to the TLS cert of the target lnd node (default "/path/tls.cert")
```