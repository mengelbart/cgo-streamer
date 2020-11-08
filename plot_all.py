#!/usr/bin/python3
# coding: utf-8

from pathlib import Path
import matplotlib.pyplot as plt
from matplotlib.backends.backend_pdf import PdfPages
import pandas as pd
import numpy as np

def plot_psnr(exps):

    fig, axs = plt.subplots(len(exps[0]), 2 * len(exps), sharex='col', figsize=(20, 20), dpi=300)
    for j in range(len(exps)):
        for i in range(len(exps[0])):
            file=exps[j][i]['path']
            df=pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 11], names=['n', 'psnr'])
            df[np.isfinite(df)]['psnr'].plot(ax=axs[i, 2*j])
            df[np.isfinite(df)]['psnr'].hist(cumulative=True, bins=len(df['psnr']), density=True, ax=axs[i, 2*j + 1])
            axs[i, 2*j].set_title(file.parts[-2].split('mkv-')[-1])
            axs[i, 2*j + 1].set_title(file.parts[-2].split('mkv-')[-1])

    fig.tight_layout()
    return fig

def plot_ssim(exps):

    fig, axs = plt.subplots(len(exps[0]), 2 * len(exps), sharex='col', figsize=(20, 20), dpi=300)
    for j in range(len(exps)):
        for i in range(len(exps[0])):
            file=exps[j][i]['path']
            df=pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 9], names=['n', 'ssim'])
            df[np.isfinite(df)]['ssim'].plot(ax=axs[i, 2*j])
            df[np.isfinite(df)]['ssim'].hist(cumulative=True, bins=len(df['ssim']), density=True, ax=axs[i, 2*j + 1])
            axs[i, 2*j].set_title(file.parts[-2].split('mkv-')[-1])
            axs[i, 2*j + 1].set_title(file.parts[-2].split('mkv-')[-1])

    fig.tight_layout()
    return fig

def psnr():
    exps = []
    for path in Path('data/').rglob('*psnr.log'):
        exps.append({ 'path': path, 'params': path.parts[-2].split('-')})

    udp_no_cc = sorted([e for e in exps if e['params'][1] == 'udp' and e['params'][3] == 'none'], key=lambda k: k['params'][2])
    udp_scream_100 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '100ms'], key=lambda k: k['params'][2])
    udp_scream_200 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '200ms'], key=lambda k: k['params'][2])
    udp_scream_300 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '300ms'], key=lambda k: k['params'][2])
    udp_scream_400 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '400ms'], key=lambda k: k['params'][2])
    udp_scream_500 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '500ms'], key=lambda k: k['params'][2])

    datagram_no_cc = sorted([e for e in exps if e['params'][1] == 'datagram' and e['params'][3] == 'none'], key=lambda k: k['params'][2])
    datagram_scream_100 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '100ms'], key=lambda k: k['params'][2])
    datagram_scream_200 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '200ms'], key=lambda k: k['params'][2])
    datagram_scream_300 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '300ms'], key=lambda k: k['params'][2])
    datagram_scream_400 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '400ms'], key=lambda k: k['params'][2])
    datagram_scream_500 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '500ms'], key=lambda k: k['params'][2])


    figures = [
        plot_psnr([udp_no_cc, datagram_no_cc]),
        plot_psnr([udp_scream_100, datagram_scream_100]),
        plot_psnr([udp_scream_200, datagram_scream_200]),
        plot_psnr([udp_scream_300, datagram_scream_300]),
#         plot_psnr([udp_scream_400, datagram_scream_400]),
#         plot_psnr([udp_scream_500, datagram_scream_500])
    ]

    with PdfPages('psnr.pdf') as pdf:
        for figure in figures:
            pdf.savefig(figure)


def ssim():
    exps = []
    for path in Path('data/').rglob('*ssim.log'):
        exps.append({ 'path': path, 'params': path.parts[-2].split('-')})

    udp_no_cc = sorted([e for e in exps if e['params'][1] == 'udp' and e['params'][3] == 'none'], key=lambda k: k['params'][2])
    udp_scream_100 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '100ms'], key=lambda k: k['params'][2])
    udp_scream_200 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '200ms'], key=lambda k: k['params'][2])
    udp_scream_300 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '300ms'], key=lambda k: k['params'][2])
    udp_scream_400 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '400ms'], key=lambda k: k['params'][2])
    udp_scream_500 = sorted([
        e for e in exps if
        e['params'][1] == 'udp' and
        e['params'][3] == 'scream' and
        e['params'][4] == '500ms'], key=lambda k: k['params'][2])

    datagram_no_cc = sorted([e for e in exps if e['params'][1] == 'datagram' and e['params'][3] == 'none'], key=lambda k: k['params'][2])
    datagram_scream_100 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '100ms'], key=lambda k: k['params'][2])
    datagram_scream_200 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '200ms'], key=lambda k: k['params'][2])
    datagram_scream_300 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '300ms'], key=lambda k: k['params'][2])
    datagram_scream_400 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '400ms'], key=lambda k: k['params'][2])
    datagram_scream_500 = sorted([
        e for e in exps if
        e['params'][1] == 'datagram' and
        e['params'][3] == 'scream' and
        e['params'][4] == '500ms'], key=lambda k: k['params'][2])

    figures = [
        plot_ssim([udp_no_cc, datagram_no_cc]),
        plot_ssim([udp_scream_100, datagram_scream_100]),
        plot_ssim([udp_scream_200, datagram_scream_200]),
        plot_ssim([udp_scream_300, datagram_scream_300]),
#         plot_ssim([udp_scream_400, datagram_scream_400]),
#         plot_ssim([udp_scream_500, datagram_scream_500])
    ]

    with PdfPages('ssim.pdf') as pdf:
        for figure in figures:
            pdf.savefig(figure)

def main():
    psnr()
    ssim()

if __name__ == "__main__":
    main()

