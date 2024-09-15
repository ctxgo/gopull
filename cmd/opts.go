package cmd

import (
	"context"
	"errors"
	"os"
	"time"

	commonFlag "github.com/containers/common/pkg/flag"
	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type globalOptions struct {
	debug              bool                    // Enable debug output
	tlsVerify          commonFlag.OptionalBool // Require HTTPS and verify certificates (for docker: and docker-daemon:)
	policyPath         string                  // Path to a signature verification policy file
	insecurePolicy     bool                    // Use an "allow everything" signature verification policy
	registriesDirPath  string                  // Path to a "registries.d" registry configuration directory
	overrideArch       string                  // Architecture to use for choosing images, instead of the runtime one
	overrideOS         string                  // OS to use for choosing images, instead of the runtime one
	overrideVariant    string                  // Architecture variant to use for choosing images, instead of the runtime one
	commandTimeout     time.Duration           // Timeout for the command execution
	registriesConfPath string                  // Path to the "registries.conf" file
	tmpDir             string                  // Path to use for big temporary files
}

// commandTimeoutContext returns a context.Context and a cancellation callback based on opts.
// The caller should usually "defer cancel()" immediately after calling this.
func (opts *globalOptions) commandTimeoutContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	var cancel context.CancelFunc = func() {}
	if opts.commandTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.commandTimeout)
	}
	return ctx, cancel
}

// getPolicyContext returns a *signature.PolicyContext based on opts.
func (opts *globalOptions) getPolicyContext() (*signature.PolicyContext, error) {
	var policy *signature.Policy // This could be cached across calls in opts.
	var err error
	if opts.insecurePolicy {
		policy = &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	} else if opts.policyPath == "" {
		policy, err = signature.DefaultPolicy(nil)
	} else {
		policy, err = signature.NewPolicyFromFile(opts.policyPath)
	}
	if err != nil {
		return nil, err
	}
	return signature.NewPolicyContext(policy)
}

// newSystemContext returns a *types.SystemContext corresponding to opts.
// It is guaranteed to return a fresh instance, so it is safe to make additional updates to it.
func (opts *globalOptions) newSystemContext() *types.SystemContext {
	ctx := &types.SystemContext{
		RegistriesDirPath:        opts.registriesDirPath,
		ArchitectureChoice:       opts.overrideArch,
		OSChoice:                 opts.overrideOS,
		VariantChoice:            opts.overrideVariant,
		SystemRegistriesConfPath: opts.registriesConfPath,
		BigFilesTemporaryDir:     opts.tmpDir,
		DockerRegistryUserAgent:  defaultUserAgent,
	}
	// DEPRECATED: We support this for backward compatibility, but override it if a per-image flag is provided.
	if opts.tlsVerify.Present() {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!opts.tlsVerify.Value())
	}
	return ctx
}

// imageOptions collects CLI flags which are the same across subcommands, but may be different for each image
// (e.g. may differ between the source and destination of a copy)
type imageOptions struct {
	dockerImageOptions
	sharedBlobDir    string // A directory to use for OCI blobs, shared across repositories
	dockerDaemonHost string // docker-daemon: host to connect to
}

// newSystemContext returns a *types.SystemContext corresponding to opts.
// It is guaranteed to return a fresh instance, so it is safe to make additional updates to it.
func (opts *imageOptions) newSystemContext() (*types.SystemContext, error) {
	// *types.SystemContext instance from globalOptions
	//  imageOptions option overrides the instance if both are present.
	ctx := opts.global.newSystemContext()
	ctx.DockerCertPath = opts.dockerCertPath
	ctx.OCISharedBlobDirPath = opts.sharedBlobDir
	ctx.AuthFilePath = opts.shared.authFilePath
	ctx.DockerDaemonHost = opts.dockerDaemonHost
	ctx.DockerDaemonCertPath = opts.dockerCertPath
	if opts.dockerImageOptions.authFilePath.Present() {
		ctx.AuthFilePath = opts.dockerImageOptions.authFilePath.Value()
	}
	if opts.deprecatedTLSVerify != nil && opts.deprecatedTLSVerify.tlsVerify.Present() {
		// If both this deprecated option and a non-deprecated option is present, we use the latter value.
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!opts.deprecatedTLSVerify.tlsVerify.Value())
	}
	if opts.tlsVerify.Present() {
		ctx.DockerDaemonInsecureSkipTLSVerify = !opts.tlsVerify.Value()
	}
	if opts.tlsVerify.Present() {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!opts.tlsVerify.Value())
	}
	if opts.credsOption.Present() && opts.noCreds {
		return nil, errors.New("creds and no-creds cannot be specified at the same time")
	}
	if opts.userName.Present() && opts.noCreds {
		return nil, errors.New("username and no-creds cannot be specified at the same time")
	}
	if opts.credsOption.Present() && opts.userName.Present() {
		return nil, errors.New("creds and username cannot be specified at the same time")
	}
	// if any of username or password is present, then both are expected to be present
	if opts.userName.Present() != opts.password.Present() {
		if opts.userName.Present() {
			return nil, errors.New("password must be specified when username is specified")
		}
		return nil, errors.New("username must be specified when password is specified")
	}
	if opts.credsOption.Present() {
		var err error
		ctx.DockerAuthConfig, err = getDockerAuth(opts.credsOption.Value())
		if err != nil {
			return nil, err
		}
	} else if opts.userName.Present() {
		ctx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: opts.userName.Value(),
			Password: opts.password.Value(),
		}
	}
	if opts.registryToken.Present() {
		ctx.DockerBearerRegistryToken = opts.registryToken.Value()
	}
	if opts.noCreds {
		ctx.DockerAuthConfig = &types.DockerAuthConfig{}
	}

	return ctx, nil
}

// dockerImageOptions collects CLI flags specific to the "docker" transport, which are
// the same across subcommands, but may be different for each image
// (e.g. may differ between the source and destination of a copy)
type dockerImageOptions struct {
	global              *globalOptions             // May be shared across several imageOptions instances.
	shared              *sharedImageOptions        // May be shared across several imageOptions instances.
	deprecatedTLSVerify *deprecatedTLSVerifyOption // May be shared across several imageOptions instances, or nil.
	authFilePath        commonFlag.OptionalString  // Path to a */containers/auth.json (prefixed version to override shared image option).
	credsOption         commonFlag.OptionalString  // username[:password] for accessing a registry
	userName            commonFlag.OptionalString  // username for accessing a registry
	password            commonFlag.OptionalString  // password for accessing a registry
	registryToken       commonFlag.OptionalString  // token to be used directly as a Bearer token when accessing the registry
	dockerCertPath      string                     // A directory using Docker-like *.{crt,cert,key} files for connecting to a registry or a daemon
	tlsVerify           commonFlag.OptionalBool    // Require HTTPS and verify certificates (for docker: and docker-daemon:)
	noCreds             bool                       // Access the registry anonymously
}

// sharedImageOptions collects CLI flags which are image-related, but do not change across images.
// This really should be a part of globalOptions, but that would break existing users of (skopeo copy --authfile=).
type sharedImageOptions struct {
	authFilePath string // Path to a */containers/auth.json
}

// imageDestOptions is a superset of imageOptions specialized for image destinations.
// Every user should call imageDestOptions.warnAboutIneffectiveOptions() as part of handling the CLI
type imageDestOptions struct {
	*imageOptions
	dirForceCompression         bool                   // Compress layers when saving to the dir: transport
	dirForceDecompression       bool                   // Decompress layers when saving to the dir: transport
	ociAcceptUncompressedLayers bool                   // Whether to accept uncompressed layers in the oci: transport
	compressionFormat           string                 // Format to use for the compression
	compressionLevel            commonFlag.OptionalInt // Level to use for the compression
	precomputeDigests           bool                   // Precompute digests to dedup layers when saving to the docker: transport
	imageDestFlagPrefix         string
}

// warnAboutIneffectiveOptions warns if any ineffective option was set by the user
// Every user should call this as part of handling the CLI
func (opts *imageDestOptions) warnAboutIneffectiveOptions(destTransport types.ImageTransport) {
	if destTransport.Name() != directory.Transport.Name() {
		if opts.dirForceCompression {
			logrus.Warnf("--%s can only be used if the destination transport is 'dir'", opts.imageDestFlagPrefix+"compress")
		}
		if opts.dirForceDecompression {
			logrus.Warnf("--%s can only be used if the destination transport is 'dir'", opts.imageDestFlagPrefix+"decompress")
		}
	}
}

// sharedImageFlags prepares a collection of CLI flags writing into sharedImageOptions, and the managed sharedImageOptions structure.
func sharedImageFlags() (pflag.FlagSet, *sharedImageOptions) {
	opts := sharedImageOptions{}
	fs := pflag.FlagSet{}
	fs.StringVar(&opts.authFilePath, "authfile", os.Getenv("REGISTRY_AUTH_FILE"), "path of the authentication file. Default is ${XDG_RUNTIME_DIR}/containers/auth.json")
	return fs, &opts
}

func retryFlags() (pflag.FlagSet, *retry.Options) {
	opts := retry.Options{}
	fs := pflag.FlagSet{}
	fs.IntVar(&opts.MaxRetry, "retry-times", 0, "the number of times to possibly retry")
	return fs, &opts
}
