package image

import (
	"fmt"
	"strings"

	"github.com/distribution/reference"
)

// ImageStruct 表示解析后的镜像信息，符合 Kubernetes 和 Docker 社区规范
type ImageStruct struct {
	Registry   string // 镜像所在的注册表 (Registry)
	Repository string // 仓库路径部分
	Name       string // 镜像名
	Tag        string // 镜像标签
	Digest     string // 镜像的 Digest
}

// ParseImageStr 解析镜像字符串，包括处理 @sha256:<digest> 的情况
func ParseImageStr(image string) (ImageStruct, error) {
	var imageInfo ImageStruct

	image, digest, err := splitDigest(image)
	if err != nil {
		return imageInfo, err
	}

	parsed, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return imageInfo, fmt.Errorf(`failed to parse image: "%s", err: %v`, image, err)
	}

	imageInfo.Registry = reference.Domain(parsed)
	imageInfo.Repository = reference.Path(parsed)
	imageInfo.Name = parseImageName(parsed)
	imageInfo.Tag = parseTag(parsed)
	imageInfo.Digest = digest

	return imageInfo, nil
}

// splitDigest 拆分出 @sha256:<digest> 并返回镜像和 digest
func splitDigest(image string) (string, string, error) {
	if strings.Contains(image, "@sha256:") {
		parts := strings.Split(image, "@sha256:")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid digest format in image string")
		}
		return parts[0], "sha256:" + parts[1], nil
	}
	return image, "", nil
}

// 解析镜像名
func parseImageName(ref reference.Named) string {
	path := reference.Path(ref)
	pathParts := strings.Split(path, "/")
	return pathParts[len(pathParts)-1]
}

// 解析 Tag
func parseTag(ref reference.Named) string {
	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ""
}
