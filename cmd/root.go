package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
	quiet   bool
)

var banner = "\n" +
	"\033[34m    .--'o o'--.\033[0m\n" +
	"\033[34m    |          |\033[0m\n" +
	"\033[34m    |  .____.  |\033[0m\n" +
	"\033[34m    '--.----.--'\033[0m\n" +
	"\033[34m    /| /|  |\\ |\\\033[0m\n" +
	"\033[34m   ^ ^ ^    ^ ^ ^\033[0m\n" +
	"\033[1;37m       SelfMD    \033[0m\n\n"

var rootCmd = &cobra.Command{
	Use:   "selfmd",
	Short: "selfmd — Auto Documentation Generator for Claude Code CLI",
	Long: banner + `Automatically generate structured, high-quality technical documentation
for any codebase — powered by Claude Code CLI.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "selfmd.yaml", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "show errors only")
}

func exitWithError(msg string) {
	fmt.Fprintln(os.Stderr, "Error: "+msg)
	os.Exit(1)
}
