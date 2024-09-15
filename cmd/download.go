package cmd

import (
	"errors"
	"fmt"
	"gopull/pkgs/image"
	"io"
	"strings"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/distribution/reference"
	"github.com/spf13/cobra"
)

type copyOptions struct {
	global              *globalOptions
	deprecatedTLSVerify *deprecatedTLSVerifyOption
	srcImage            *imageOptions
	destImage           *imageDestOptions
	retryOpts           *retry.Options
	format              commonFlag.OptionalString // Force conversion of the image to a specified format
	quiet               bool                      // Suppress output information when copying images
	outFile             string
	addTag              string // For docker-archive: destinations, in addition to the name:tag specified as destination, also add these

}

func download(global *globalOptions) *cobra.Command {
	sharedFlags, sharedOpts := sharedImageFlags()
	deprecatedTLSVerifyFlags, deprecatedTLSVerifyOpt := deprecatedTLSVerifyFlags()
	srcFlags, srcOpts := imageFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "src-", "screds")
	destFlags, destOpts := imageDestFlags(global, sharedOpts, deprecatedTLSVerifyOpt, "dest-", "dcreds")
	retryFlags, retryOpts := retryFlags()
	opts := copyOptions{global: global,
		deprecatedTLSVerify: deprecatedTLSVerifyOpt,
		srcImage:            srcOpts,
		destImage:           destOpts,
		retryOpts:           retryOpts,
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

func (opts *copyOptions) run(args []string, stdout io.Writer) (retErr error) {
	if len(args) != 1 {
		return errorShouldDisplayUsage{errors.New("image is required")}
	}
	opts.deprecatedTLSVerify.warnIfUsed([]string{"--src-tls-verify", "--dest-tls-verify"})
	imageNames := args

	policyContext, err := opts.global.getPolicyContext()
	if err != nil {
		return fmt.Errorf("Error loading trust policy: %v", err)
	}
	defer func() {
		if err := policyContext.Destroy(); err != nil {
			retErr = noteCloseFailure(retErr, "tearing down policy context", err)
		}
	}()

	srcRef, err := alltransports.ParseImageName("docker://" + imageNames[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", imageNames[0], err)
	}

	parsedImage, err := image.ParseImageStr(imageNames[0])
	if err != nil {
		return fmt.Errorf("parse image faild name %s: %v", imageNames[0], err)
	}
	dest := "docker-archive:"
	if opts.outFile != "" {
		dest += opts.outFile
	} else {
		dest += getDefaultImageTarName(parsedImage)
	}

	destRef, err := alltransports.ParseImageName(dest)
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", dest, err)
	}

	sourceCtx, err := opts.srcImage.newSystemContext()
	if err != nil {
		return err
	}

	destinationCtx, err := opts.destImage.newSystemContext()
	if err != nil {
		return err
	}

	var destTag string

	if opts.addTag == "" {
		if parsedImage.Digest != "" {
			return fmt.Errorf(`源镜像名称(redis@%s...) 包含 digest, 
			需要显示的设置目标tag, 请使用 -t 或者 --tag 参数来设置`, parsedImage.Digest[:19])
		}
		destTag = getDestTagFormImageStruct(parsedImage)

	} else {
		destTag = getDestTagFromString(opts.addTag)

	}
	if err := addTag(destinationCtx, destTag); err != nil {
		return fmt.Errorf(`添加目标tag失败,name:%s err:%v`, opts.addTag, err)
	}

	var manifestType string
	if opts.format.Present() {
		manifestType, err = parseManifestFormat(opts.format.Value())
		if err != nil {
			return err
		}
	}

	ctx, cancel := opts.global.commandTimeoutContext()
	defer cancel()

	if opts.quiet {
		stdout = nil
	}

	opts.destImage.warnAboutIneffectiveOptions(destRef.Transport())

	return retry.IfNecessary(ctx, func() error {
		_, err := copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
			ReportWriter:          stdout,
			SourceCtx:             sourceCtx,
			DestinationCtx:        destinationCtx,
			ForceManifestMIMEType: manifestType,
			ImageListSelection:    copy.CopySystemImage,
		})
		if err != nil {
			return err
		}

		return nil
	}, opts.retryOpts)
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
