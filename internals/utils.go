package internals

import "github.com/docker/distribution/reference"

func DockerImageDomain(image string) string {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		panic(err)
	}
	return reference.Domain(ref)
}

func Ptr[T any](v T) *T {
	return &v
}
