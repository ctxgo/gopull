package cmd

import (
	"fmt"
	"io"
	"strings"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/spf13/cobra"
)

type pushOptions struct {
	*copyOptions
	destTag string
}

func push(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	deprecatedTLSVerifyFlags, deprecatedTLSVerifyOpt := deprecatedTLSVerifyFlags()
	srcFlags, srcOpts := imageFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "src-", "screds")
	destFlags, destOpts := imageDestFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "dest-", "dcreds")
	retryFlags, retryOpts := retryFlags()
	opts := pushOptions{
		copyOptions: &copyOptions{
			global:              global,
			deprecatedTLSVerify: deprecatedTLSVerifyOpt,
			srcImage:            srcOpts,
			destImage:           destOpts,
			retryOpts:           retryOpts,
		},
	}
	cmd := &cobra.Command{
		Use:   "push [command options] IMAGE ",
		Short: "push an image",
		Long: fmt.Sprintf(`Container "IMAGE-NAME" uses a "transport":"details" format.

Supported transports:
%s

See skopeo(1) section "IMAGE NAMES" for the expected format
`, strings.Join(transports.ListNames(), ", ")),
		RunE: commandAction(opts.run),
		Example: `gopull push redis
gopull push redis -t example.harbor.org/redis:v1
`,
		ValidArgsFunction: autocompleteSupportedTransports,
	}
	adjustUsage(cmd)
	flags := cmd.Flags()
	flags.AddFlagSet(&sharedFlags)
	flags.AddFlagSet(&deprecatedTLSVerifyFlags)
	flags.AddFlagSet(&srcFlags)
	flags.AddFlagSet(&destFlags)
	flags.AddFlagSet(&retryFlags)
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Suppress output information when copying images")
	flags.VarP(commonFlag.NewOptionalStringValue(&opts.format), "format", "f", `MANIFEST TYPE (oci, v2s1, or v2s2) to use in the destination (default is manifest type of source, with fallbacks)`)
	flags.StringVarP(&opts.destTag, "--tag", "t", "", "Push destination")
	return cmd
}

func (opts *pushOptions) run(args []string, stdout io.Writer) error {
	return opts.execCopy(args, stdout, opts.buildSrcRef, opts.buildDestRef)
}

func (opts *pushOptions) buildSrcRef(imageName string) (types.ImageReference, *types.SystemContext, error) {

	srcRef, err := alltransports.ParseImageName("docker-daemon:" + imageName)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid source name %s: %v", imageName, err)
	}
	sourceCtx, err := opts.srcImage.newSystemContext()
	if err != nil {
		return nil, nil, err
	}
	return srcRef, sourceCtx, nil
}

func (opts *pushOptions) buildDestRef(imageName string) (types.ImageReference, *types.SystemContext, error) {

	dest := "docker://" + imageName
	if opts.destTag != "" {
		dest = "docker://" + opts.destTag
	}

	destRef, err := alltransports.ParseImageName(dest)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid destination name %s: %v", dest, err)
	}

	destCtx, err := opts.destImage.newSystemContext()
	if err != nil {
		return nil, nil, err
	}
	return destRef, destCtx, nil
}
