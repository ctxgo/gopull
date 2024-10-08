package cmd

import (
	"fmt"
	"gopull/pkgs/image"
	"io"
	"strings"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/spf13/cobra"
)

type pullOptions struct {
	*copyOptions
	addTag string // For docker-archive: destinations, in addition to the name:tag specified as destination, also add these

}

func pull(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	deprecatedTLSVerifyFlags, deprecatedTLSVerifyOpt := deprecatedTLSVerifyFlags()
	srcFlags, srcOpts := imageFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "src-", "screds")
	destFlags, destOpts := imageDestFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "dest-", "dcreds")
	retryFlags, retryOpts := retryFlags()
	opts := pullOptions{
		copyOptions: &copyOptions{
			global:              global,
			deprecatedTLSVerify: deprecatedTLSVerifyOpt,
			srcImage:            srcOpts,
			destImage:           destOpts,
			retryOpts:           retryOpts,
		},
	}
	cmd := &cobra.Command{
		Use:   "pull [command options] IMAGE ",
		Short: "pull an image to docker",
		Long: fmt.Sprintf(`Container "IMAGE-NAME" uses a "transport":"details" format.

Supported transports:
%s

See skopeo(1) section "IMAGE NAMES" for the expected format
`, strings.Join(transports.ListNames(), ", ")),
		RunE:              commandAction(opts.run),
		Example:           `gopull pull redis`,
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
	flags.StringVarP(&opts.addTag, "tag", "t", "", "set dest tag")
	return cmd
}

func (opts *pullOptions) run(args []string, stdout io.Writer) error {
	return opts.execCopy(args, stdout, opts.buildSrcRef, opts.buildDestRef)
}

func (opts *pullOptions) buildSrcRef(imageName string) (types.ImageReference, *types.SystemContext, error) {

	srcRef, err := alltransports.ParseImageName("docker://" + imageName)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid source name %s: %v", imageName, err)
	}
	sourceCtx, err := opts.srcImage.newSystemContext()
	if err != nil {
		return nil, nil, err
	}
	return srcRef, sourceCtx, nil
}

func (opts *pullOptions) buildDestRef(imageName string) (types.ImageReference, *types.SystemContext, error) {

	parsedImage, err := image.ParseImageStr(imageName)
	if err != nil {
		return nil, nil, fmt.Errorf("parse image faild name %s: %v", imageName, err)
	}
	dest := "docker-daemon:"

	destTag, err := buildDestTag(parsedImage, opts.addTag)
	if err != nil {
		return nil, nil, err
	}
	dest += destTag

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

func buildDestTag(parsedImage image.ImageStruct, addTag string) (string, error) {
	if addTag != "" {
		return getDestTagFromString(addTag), nil
	}

	if parsedImage.Digest == "" {
		return getDestTagFormImageStruct(parsedImage), nil
	}

	return "", fmt.Errorf(`源镜像名称(redis@%s...) 包含 digest, 
	需要显示的设置目标tag, 请使用 -t 或者 --tag 参数来设置`, parsedImage.Digest[:19])
}
