package cmd

import (
	"errors"
	"fmt"
	"io"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/types"
)

type copyOptions struct {
	global              *globalOptions
	deprecatedTLSVerify *deprecatedTLSVerifyOption
	srcImage            *imageOptions
	destImage           *imageDestOptions
	retryOpts           *retry.Options
	format              commonFlag.OptionalString // Force conversion of the image to a specified format
	quiet               bool                      // Suppress output information when copying images
}

type buildImageRefer func(string) (types.ImageReference, *types.SystemContext, error)

func (opts *copyOptions) execCopy(args []string, stdout io.Writer, s buildImageRefer, d buildImageRefer) (retErr error) {
	if len(args) != 1 {
		return errorShouldDisplayUsage{errors.New("image is required")}
	}
	opts.deprecatedTLSVerify.warnIfUsed([]string{"--src-tls-verify", "--dest-tls-verify"})
	imageName := args[0]

	policyContext, err := opts.global.getPolicyContext()
	if err != nil {
		return fmt.Errorf("error loading trust policy: %v", err)
	}
	defer func() {
		if err := policyContext.Destroy(); err != nil {
			retErr = noteCloseFailure(retErr, "tearing down policy context", err)
		}
	}()

	srcRef, sourceCtx, err := s(imageName)
	if err != nil {
		return err
	}

	destRef, destCtx, err := d(imageName)
	if err != nil {
		return err
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
			DestinationCtx:        destCtx,
			ForceManifestMIMEType: manifestType,
			ImageListSelection:    copy.CopySystemImage,
		})
		if err != nil {
			return err
		}

		return nil
	}, opts.retryOpts)
}
