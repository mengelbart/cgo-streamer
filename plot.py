#!/usr/bin/python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import glob
import os

def plot_ssim(file, title):
    df=pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 9], names=['n', 'ssim'])

    fig, axes = plt.subplots(nrows=1, ncols=2)

    df[np.isfinite(df)]['ssim'].plot(ax=axes[0])
    df[np.isfinite(df)]['ssim'].hist(cumulative=True, bins=len(df['ssim'].unique()), ax=axes[1])

    plt.suptitle('ssim: ' + title)
    plt.savefig('ssim.png')

def plot_psnr(file, title):
    df=pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 11], names=['n', 'psnr'])

    fig, axes = plt.subplots(nrows=1, ncols=2)

    df[np.isfinite(df)]['psnr'].plot(ax=axes[0])
    df[np.isfinite(df)]['psnr'].hist(cumulative=True, bins=len(df['psnr'].unique()), ax=axes[1])

    plt.suptitle('psnr: ' + title)
    plt.savefig('psnr.png')

def plot_scream(file, title):
    df=pd.read_csv('scream.log', sep="\s+|\t+|\s+\t+|\t+\s+", engine='python',
            names=['date', 'time', 'queueLen', 'cwnd', 'bytesInFlight', 'fastStart', 'queueDelay', 'targetBitrate', 'rateTransmitted'])
    df.plot(x='time', y=['queueLen', 'cwnd', 'bytesInFlight', 'targetBitrate', 'rateTransmitted'])
    plt.savefig('scream.png')

def main():
    title=os.getcwd().split('/')[-1]
    plot_ssim("ssim.log", title)
    plot_psnr("psnr.log", title)
    if os.path.isfile("scream.log"):
        plot_scream("scream.log", title)


if __name__ == "__main__":
    main()
