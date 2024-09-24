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
	"github.com/distribution/reference"
	"github.com/spf13/cobra"
)

type downloadOptions struct {
	*pullOptions
	outFile string
}

func download(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	deprecatedTLSVerifyFlags, deprecatedTLSVerifyOpt := deprecatedTLSVerifyFlags()
	srcFlags, srcOpts := imageFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "src-", "screds")
	destFlags, destOpts := imageDestFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "dest-", "dcreds")
	retryFlags, retryOpts := retryFlags()
	opts := downloadOptions{
		pullOptions: &pullOptions{
			copyOptions: &copyOptions{
				global:              global,
				deprecatedTLSVerify: deprecatedTLSVerifyOpt,
				srcImage:            srcOpts,
				destImage:           destOpts,
				retryOpts:           retryOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:   "download [command options] IMAGE ",
		Short: "download an image",
		Long: fmt.Sprintf(`Container "IMAGE-NAME" uses a "transport":"details" format.

Supported transports:
%s

See skopeo(1) section "IMAGE NAMES" for the expected format
`, strings.Join(transports.ListNames(), ", ")),
		RunE:              commandAction(opts.run),
		Example:           `gopull download redis`,
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
	flags.StringVarP(&opts.outFile, "outfile", "o", "", "Read a passphrase for signing an image from `PATH`")
	flags.StringVarP(&opts.addTag, "tag", "t", "", "set dest tag ")
	return cmd
}

func (opts *downloadOptions) run(args []string, stdout io.Writer) error {
	return opts.pullOptions.execCopy(args, stdout, opts.buildSrcRef, opts.buildDestRef)
}

func (opts *downloadOptions) buildDestRef(imageName string) (types.ImageReference, *types.SystemContext, error) {

	parsedImage, err := image.ParseImageStr(imageName)
	if err != nil {
		return nil, nil, fmt.Errorf("parse image faild name %s: %v", imageName, err)
	}

	dest := "docker-archive:"
	if opts.outFile != "" {
		dest += opts.outFile
	} else {
		dest += getDefaultImageTarName(parsedImage)
	}

	destRef, err := alltransports.ParseImageName(dest)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid destination name %s: %v", dest, err)
	}

	destCtx, err := opts.destImage.newSystemContext()
	if err != nil {
		return nil, nil, err
	}

	var destTag string

	destTag, err = buildDestTag(parsedImage, opts.addTag)
	if err != nil {
		return nil, nil, err
	}

	if err := addTag(destCtx, destTag); err != nil {
		return nil, nil, fmt.Errorf(`添加目标tag失败,name:%s err:%v`, opts.addTag, err)
	}
	return destRef, destCtx, nil
}

func addTag(sysCtx *types.SystemContext, tag string) error {
	ref, err := reference.ParseNormalizedNamed(tag)
	if err != nil {
		return fmt.Errorf("error parsing tag %v", err)
	}
	namedTagged, isNamedTagged := ref.(reference.NamedTagged)
	if !isNamedTagged {
		return fmt.Errorf("dest must be a tagged reference")
	}
	sysCtx.DockerArchiveAdditionalTags = append(sysCtx.DockerArchiveAdditionalTags, namedTagged)
	return nil
}
