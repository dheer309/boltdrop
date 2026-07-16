package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bolt",
	Short: "A lightning fast file transfer tool",
	Long: `boltdrop is a CLI file transfer tool that helps you transfer your files without the need of a working internet.

Share files locally, just by being on the same network, faster than your average internet connection.`,
}

var sendCmd = &cobra.Command{
	Use:   "send [filepath]",
	Short: "Starts file send",
	Long:  "Start a file send by running this command with the file path",
	Args:  cobra.ExactArgs(1),
	Run:   sendFile,
}

var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Starts receiving a file",
	Long:  "Let the sender discover your device by this command.",
	Run:   receiveFile,
}

func init() {
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(receiveCmd)
}

func main() {
	rootCmd.Execute()
}
