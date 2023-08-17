# ctmon

> A certificate transparency log monitor

## Disclaimer

There are alternatives to just find the certificates already online, like [crt.sh](https://crt.sh), [Entrust](https://ui.ctsearch.entrust.com/ui/ctsearchui), [sslmate](https://sslmate.com/ct_search_api/) and [others](https://community.letsencrypt.org/t/certificate-transparency-search-resources/203368), if you still need the certificates for a special use case please see the section [Remarks](#remarks) of this document.

## About

This application is a Proof of Concept application to download certificates for a Certificate Transparency Log.

It's guaranted to grab all certificates in sequence for any logs that

## Usage Examples

Print all certificates for any Brazilian domain:

```sh
ctmon download --regex '\.br$'
```

Download certificates using multiple IP addresses to avoid rate limits (on my tests it saturated a Gigabit Link with two IPs):

```sh
ctmon download --ip=0.0.0.100 --ip=0.0.0.200
```

Download certiticates and save it to a Postgresql Database:

```sh
ctmon download --save
ctmon download --save-bulk
```

<a id="Remarks">## Remarks</a>

### Performance configuration

When downloading certificates you are most likely to be throated by the CT server than to hit limits of this tool.

To be friendly with the CT server logs, this tool respects (and requires it to be configured on the `state.json` file) the [coerced get-entries](https://community.letsencrypt.org/t/enabling-coerced-get-entries/114436) paging and only download pages at correct start/end boundaries.

### Network Considerations

If you are in sync with the current tree heads, you can expect an average 10 to 30 MB/s constantly.

Out of sync (if you run this tool only for certain hours or when downloading the entire tree) on the default configuration using only one IP, your bandwidth consumption will be several hundred megabits for downloading historical data before getting rate limited. For those downloading historical data, a option to include several IPs was added

### Storage Considerations

If you plan to store certificates, Postgresql are one of the best options, please check [Database Considerations](#database-considerations) sections.

If you parse the certificate data, then may Elastic Search / Open Search can be used.

Each DER certificate is about 1.5kB and the current valid (not expired) certificates surpaces a billion and growing more than [a million per day](https://letsencrypt.org/stats/), requiring at least a few terabytes of rapid storage (NVME recommended) to deal with it.

Due to small size, compression does not help to much for storing raw certificates with an exception which is using [zstardard](https://facebook.github.io/zstd/) with a custom dictionary, with can achieve about 50% compression on each file.

### Database Considerations

We implement a basic but usable storage of certificate on Postgresql, thanks to [libx509pq](https://github.com/crtsh/libx509pq) it is very efficient in terms of search and space. I have an extra optimization for it called [libz509pq](https://github.com/sergiogarciadev/libz509pq) with very _zimilar_ API (mostly a `z` instead of `x`), but with compression using a special dictionary for DER certificates with can achieve about 50% compression on each certificate, accounting for about 20-30% reduction of storage needs and achieving better performance on queries that requires table scans and other operation that are IO bound.

The certificate table have a unique key based on certificate SHA-256 digest that will ensure that the same certificate are not stored twice, but it will continue to store both the pre-certificate and it's corresponding certificate if both are saved to logs.

A very basic bulk implementation exists to speed up data ingestion but it still need vastly optimizations to be usable in real scenarios.

## Developer Container

The configuration for VS Code Containers exists on this repository with everything you need to get started exploring this tool.
