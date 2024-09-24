package cmd

import (
	"fmt"
	"strings"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/storage/pkg/reexec"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// requireSubcommand returns an error if no sub command is provided
// This was copied from podman: `github.com/containers/podman/cmd/podman/validate/args.go
// Some small style changes to match skopeo were applied, but try to apply any
// bugfixes there first.
func requireSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		suggestions := cmd.SuggestionsFor(args[0])
		if len(suggestions) == 0 {
			return fmt.Errorf("Unrecognized command `%[1]s %[2]s`\nTry '%[1]s --help' for more information", cmd.CommandPath(), args[0])
		}
		return fmt.Errorf("Unrecognized command `%[1]s %[2]s`\n\nDid you mean this?\n\t%[3]s\n\nTry '%[1]s --help' for more information", cmd.CommandPath(), args[0], strings.Join(suggestions, "\n\t"))
	}
	return fmt.Errorf("Missing command '%[1]s COMMAND'\nTry '%[1]s --help' for more information", cmd.CommandPath())
}

// createApp returns a cobra.Command, and the underlying globalOptions object, to be run or tested.
func createApp() (*cobra.Command, *globalOptions) {
	opts := globalOptions{}

	rootCommand := &cobra.Command{
		Use:               "gopull",
		Long:              "Various operations with container images and container image registries",
		RunE:              requireSubcommand,
		PersistentPreRunE: opts.before,
		SilenceUsage:      true,
		SilenceErrors:     true,
		// Hide the completion command which is provided by cobra
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
		// This is documented to parse "local" (non-PersistentFlags) flags of parent commands before
		// running subcommands and handling their options. We don't really run into such cases,
		// because all of our flags on rootCommand are in PersistentFlags, except for the deprecated --tls-verify;
		// in that case we need TraverseChildren so that we can distinguish between
		// (skopeo --tls-verify inspect) (causes a warning) and (skopeo inspect --tls-verify) (no warning).
		TraverseChildren: true,
	}
	if gitCommit != "" {
		rootCommand.Version = fmt.Sprintf("%s commit: %s", version, gitCommit)
	} else {
		rootCommand.Version = version
	}
	// Override default `--version` global flag to enable `-v` shorthand
	rootCommand.Flags().BoolP("version", "v", false, "Version for Skopeo")
	rootCommand.PersistentFlags().BoolVar(&opts.debug, "debug", false, "enable debug output")
	rootCommand.PersistentFlags().StringVar(&opts.policyPath, "policy", "", "Path to a trust policy file")
	rootCommand.PersistentFlags().BoolVar(&opts.insecurePolicy, "insecure-policy", false, "run the tool without any policy check")
	rootCommand.PersistentFlags().StringVar(&opts.registriesDirPath, "registries.d", "", "use registry configuration files in `DIR` (e.g. for container signature storage)")
	rootCommand.PersistentFlags().StringVar(&opts.overrideArch, "override-arch", "", "use `ARCH` instead of the architecture of the machine for choosing images")
	rootCommand.PersistentFlags().StringVar(&opts.overrideOS, "override-os", "", "use `OS` instead of the running OS for choosing images")
	rootCommand.PersistentFlags().StringVar(&opts.overrideVariant, "override-variant", "", "use `VARIANT` instead of the running architecture variant for choosing images")
	rootCommand.PersistentFlags().DurationVar(&opts.commandTimeout, "command-timeout", 0, "timeout for the command execution")
	rootCommand.PersistentFlags().StringVar(&opts.tmpDir, "tmpdir", "", "directory used to store temporary files")
	flag := commonFlag.OptionalBoolFlag(rootCommand.Flags(), &opts.tlsVerify, "tls-verify", "Require HTTPS and verify certificates when accessing the registry")
	flag.Hidden = true
	rootCommand.AddCommand(
		download(&opts),
		pull(&opts),
		push(&opts),
		inspectCmd(&opts),
		loginCmd(&opts),
		logoutCmd(&opts),
	)
	return rootCommand, &opts
}

// before is run by the cli package for any command, before running the command-specific handler.
func (opts *globalOptions) before(cmd *cobra.Command, args []string) error {
	if opts.debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if opts.tlsVerify.Present() {
		logrus.Warn("'--tls-verify' is deprecated, please set this on the specific subcommand")
	}
	return nil
}

func Execute() {
	if reexec.Init() {
		return
	}
	rootCmd, _ := createApp()
	if err := rootCmd.Execute(); err != nil {
		if isNotFoundImageError(err) {
			logrus.StandardLogger().Log(logrus.FatalLevel, err)
			logrus.Exit(2)
		}
		logrus.Fatal(err)
	}
}
