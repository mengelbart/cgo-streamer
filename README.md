# Real time media over QUIC

There are various options to use QUIC for real time media e.g. for video conferencing.
This project contains an implementation of different strategies for transmitting real-time videos over QUIC and UDP along with a simple Benchmarking tool that can be used to evaluate the performance by analysing the quality of the received video.
The project uses CGO to wrap gstreamer for video processing and the [quic-go](https://github.com/lucas-clemente/quic-go) implementation of the QUIC protocol. 
The quic-go implementation has been [forked](https://github.com/mengelbart/quic-go) and extended by an implementation of the datagram draft and all quic internal congestion control has been disabled in order to implement separate congestion control in the application (see [Congestion Control](#congestion-control)).

## Installation

Make sure you have go and [gstreamer](https://gstreamer.freedesktop.org/documentation/installing/on-linux.html) installed, then just run `make`.

To run a video server that serves a simple testvideo:

```shell script
./qrt serve
```

then, in a second terminal, start a client to receive and display the video:

```shell script
./qrt stream
```

The commands support a range of different options to configure the transport parameters which can be listed with

```shell script
./qrt help [command]
```

## Congestion Control

Currently, the SCReAM congestion control algorithm implementation from [EricssonResearch](https://github.com/EricssonResearch/scream/) via another [CGO wrapper](https://github.com/mengelbart/scream-go) is supported.
SCReAM congestion control can be enabled using the `-s` flag on `server` and `stream` commands.

## Benchmarking

The `bench` command can be used to run and evaluate a number of setups automatically.
Before running this command you need to create some virtual interfaces which can be done using the `vnetns.sh` script with `up` as parameter (`down` to clean up afterwards).
The program will create a directory hierarchy containing information about each test run and evaluation metrics like the SCReAM statistics and [SSIM](https://en.wikipedia.org/wiki/Structural_similarity) and [PSNR](https://en.wikipedia.org/wiki/Peak_signal-to-noise_ratio) statistics for each run.
The python script `plot_all.py` can be used to plot some visualizations of these statistics.