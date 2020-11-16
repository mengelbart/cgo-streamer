#!/usr/bin/python3
# coding: utf-8

from pathlib import Path
import matplotlib.pyplot as plt
from matplotlib.backends.backend_pdf import PdfPages
import pandas as pd
import numpy as np
import argparse
import glob
import re
import itertools
import matplotlib as mpl
import os
mpl.rcParams['figure.dpi'] = 100

FILE = 0
TRANSPORT = 1
BANDWIDTH = 2
CONGESTION_CONTROL = 3
FEEDBACK_FREQUENCY = 4


def get_log_paths(path, metric):
    exps = []
    for log in Path(path).rglob('*' + metric + '.log'):
        exps.append({ 'path': log, 'params': log.parts[-2].split('-')})
    return exps

def get_df(file, metric, col_names=[]):
    if metric == 'ssim':
        return pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 9], names=['n', metric])
    elif metric == 'psnr':
        return pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 11], names=['n', metric])
    elif metric == 'box_ssim':
        return pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[9], names=col_names)
    elif metric == 'box_psnr':
        return pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[11], names=col_names)

def plot_scream(exps):
    fig, axs = plt.subplots(3 * len(exps[0]), len(exps), sharex=True, sharey='row', figsize=(30, 30), dpi=300)
    for j in range(len(exps)):
            for i in range(len(exps[0])):
                file=exps[j][i]['path']
                df=pd.read_csv(file, sep="\s+|\t+|\s+\t+|\t+\s+", engine='python',
                    names=['time', 'queueLen', 'cwnd', 'bytesInFlight', 'fastStart', 'queueDelay', 'targetBitrate', 'rateTransmitted'])

                df.sort_values('time').plot(x='time', y=['cwnd', 'bytesInFlight'], ax=axs[i * 3, j])
                df.sort_values('time').plot(x='time', y=['targetBitrate', 'rateTransmitted'], ax=axs[i * 3 + 1, j])
                df.sort_values('time').plot(x='time', y=['queueLen'], ax=axs[i * 3 + 2, j])

                axs[i * 3, j].set_title(file.parts[-2].split('mkv-')[-1])

    plot.tight_layout()
    cdf.tight_layout()
    return fig

def plot_metric(exps, base_path, metric):

    plot, plot_axs = plt.subplots(len(exps[0]), len(exps), sharex=True, sharey=True, figsize=(30, 30), dpi=300)
    cdf, cdf_axs = plt.subplots(len(exps[0]), len(exps), sharex=True, sharey=True, figsize=(30, 30), dpi=300)
    for j in range(len(exps)):
        for i in range(len(exps[0])):
            c = exps[j][i]
            print(c)
            name = '{}-{}-{}-{}-{}'.format(
                c[FILE],
                c[TRANSPORT],
                c[BANDWIDTH],
                c[CONGESTION_CONTROL],
                c[FEEDBACK_FREQUENCY]
            )
            file = Path(base_path + name + '/'+ metric + '.log')
            df = get_df(file, metric)
            df[np.isfinite(df)][metric].plot(ax=plot_axs[i, j])
            df[np.isfinite(df)][metric].hist(cumulative=True, bins=len(df[metric]), density=True, ax=cdf_axs[i, j])
            if metric == 'ssim':
                plot_axs.set_ylim([-1, 1])
            elif metric == 'psnr':
                plot_axs.set_ylim([0, 100])
            plot_axs[i, j].set_title('plot: ' + name)
            cdf_axs[i, j].set_title('cdf: ' + name)

    plot.tight_layout()
    cdf.tight_layout()
    return plot, cdf

def boxplot(exps, base_path, metric, ms_per_plot):
    rows = int(len(exps[0])/ms_per_plot)
    plot, plot_axs = plt.subplots(nrows=rows, ncols=len(exps), figsize=(15, 60), dpi=300)
    for i in range(rows):
        for j in range(len(exps)):
            dfs = []
            for k in range(ms_per_plot):
                c = exps[j][i * ms_per_plot + k]
                name = '{}-{}-{}-{}-{}'.format(
                    c[FILE],
                    c[TRANSPORT],
                    c[BANDWIDTH],
                    c[CONGESTION_CONTROL],
                    c[FEEDBACK_FREQUENCY]
                )
                file = Path(base_path + name + '/' + metric + '.log')
                col_name = c[FEEDBACK_FREQUENCY] if c[FEEDBACK_FREQUENCY] != '0s' else 'none'
                df = get_df(file, 'box_' + metric, col_names=[col_name])
                dfs.append(df)

            frame = pd.concat(dfs, axis=1)
            axes = frame.boxplot(rot=90, figsize=(3, 6), ax=plot_axs[i, j])
            if metric == 'ssim':
                axes.set_ylim([-1, 1])
            elif metric == 'psnr':
                axes.set_ylim([0, 100])
            plot_axs[i, j].set_title('{}-{}-{}'.format(c[FILE], c[TRANSPORT], str(c[BANDWIDTH] / 1000000) + 'Mb/s'))

    plot.tight_layout()
    return plot


def main():
    parser = argparse.ArgumentParser('Plot cgo-streamer benchmarks')
    parser.add_argument('path')
    args = parser.parse_args()
    BASE_PATH = args.path

    combinations = get_log_paths(Path(BASE_PATH), 'ssim')

    files = list(set([combi['params'][FILE] for combi in combinations]))
    transports = list(set([combi['params'][TRANSPORT] for combi in combinations]))
    bandwidths = sorted(list(set([int(combi['params'][BANDWIDTH]) for combi in combinations])))
    congestion_controls = sorted(list(set([combi['params'][CONGESTION_CONTROL] for combi in combinations])))
    feedback_frequencies = sorted(list(set([combi['params'][FEEDBACK_FREQUENCY] for combi in combinations])))

    product = sorted(list(itertools.product(*[
        files,
        transports,
        bandwidths,
        congestion_controls,
        feedback_frequencies
    ])), key=lambda k: (k[0], k[1], k[2], k[3], int(re.split('ms|s', k[4])[0])))


    product = [f for f in product if os.path.isdir(
        BASE_PATH + '{}-{}-{}-{}-{}'.format(f[FILE], f[TRANSPORT], f[BANDWIDTH], f[CONGESTION_CONTROL], f[FEEDBACK_FREQUENCY])
        )]

    udp = [c for c in product if c[TRANSPORT] == 'udp']
    datagram = [c for c in product if c[TRANSPORT] == 'datagram']
    streamperframe = [c for c in product if c[TRANSPORT] == 'streamperframe']

    for f in files:
        for m in ['ssim', 'psnr']:
            with PdfPages(f + '-' + m + '.pdf') as pdf:
                for b in bandwidths:
                    udp_b = [u for u in udp if u[BANDWIDTH] == b and u[FILE] == f]
                    datagram_b = [u for u in datagram if u[BANDWIDTH] == b and u[FILE] == f]
                    streamperframe_b = [u for u in streamperframe if u[BANDWIDTH] == b and u[FILE] == f]
                    plot, cdf = plot_metric([udp_b, datagram_b, streamperframe_b], BASE_PATH, m)
                    pdf.savefig(plot)
                    pdf.savefig(cdf)
                    plt.close(plot)
                    plt.close(cdf)

            with PdfPages(f + '-' + m + '-box.pdf') as pdf:
                udp_f = [u for u in udp if u[FILE] == f]
                datagram_f = [u for u in datagram if u[FILE] == f]
                streamperframe_f = [u for u in streamperframe if u[FILE] == f]
                fig = boxplot([udp_f, datagram_f, streamperframe_f], BASE_PATH, m, len(feedback_frequencies))
                pdf.savefig(fig)
                plt.close(fig)

if __name__ == "__main__":
    main()

