package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/mengelbart/cgo-streamer/benchmark"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(uploadCmd)
}

var uploadCmd = &cobra.Command{
	Use: "upload",
	RunE: func(cmd *cobra.Command, args []string) error {
		return upload()
	},
}

func upload() error {
	uploader, err := benchmark.NewUploader()
	if err != nil {
		return err
	}
	err = filepath.Walk("./data", func(path string, info os.FileInfo, err error) error {
		if info.Name() == "config.json" {
			err := uploader.Upload(filepath.Dir(path))
			if err != nil {
				log.Printf("failed to upload experiment: %v\n", path)
			}
		}
		return nil
	})
	return nil
}
