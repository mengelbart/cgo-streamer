#!/usr/bin/python3

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import glob

def plot_ssim(file):
    df=pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 9], names=['n', 'ssim'])

    fig, axes = plt.subplots(nrows=1, ncols=2)

    df[np.isfinite(df)]['ssim'].plot(ax=axes[0])
    df[np.isfinite(df)]['ssim'].hist(cumulative=True, bins=len(df['ssim'].unique()), ax=axes[1])

    plt.suptitle(file)
    plt.savefig('ssim.png')

def plot_psnr(file):
    df=pd.read_csv(file, sep=r'[\s:]', engine='python', usecols=[1, 11], names=['n', 'psnr'])

    fig, axes = plt.subplots(nrows=1, ncols=2)

    df[np.isfinite(df)]['psnr'].plot(ax=axes[0])
    df[np.isfinite(df)]['psnr'].hist(cumulative=True, bins=len(df['psnr'].unique()), ax=axes[1])

    plt.suptitle(file)
    plt.savefig('psnr.png')

def main():
    plot_ssim("ssim.log")
    plot_psnr("psnr.log")


if __name__ == "__main__":
    main()
