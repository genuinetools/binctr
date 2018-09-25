package clair

import (
	"fmt"
	"strings"

	"github.com/coreos/clair/api/v3/clairpb"
	"github.com/docker/distribution"
	"github.com/genuinetools/reg/registry"
)

// NewClairLayer will form a layer struct required for a clair scan.
func (c *Clair) NewClairLayer(r *registry.Registry, image string, fsLayers map[int]distribution.Descriptor, index int) (*Layer, error) {
	var parentName string
	if index < len(fsLayers)-1 {
		parentName = fsLayers[index+1].Digest.String()
	}

	// Form the path.
	p := strings.Join([]string{r.URL, "v2", image, "blobs", fsLayers[index].Digest.String()}, "/")

	// Get the headers.
	h, err := r.Headers(p)
	if err != nil {
		return nil, err
	}

	return &Layer{
		Name:       fsLayers[index].Digest.String(),
		Path:       p,
		ParentName: parentName,
		Format:     "Docker",
		Headers:    h,
	}, nil
}

// NewClairV3Layer will form a layer struct required for a clair scan.
func (c *Clair) NewClairV3Layer(r *registry.Registry, image string, fsLayer distribution.Descriptor) (*clairpb.PostAncestryRequest_PostLayer, error) {
	// Form the path.
	p := strings.Join([]string{r.URL, "v2", image, "blobs", fsLayer.Digest.String()}, "/")

	// Get the headers.
	h, err := r.Headers(p)
	if err != nil {
		return nil, err
	}

	return &clairpb.PostAncestryRequest_PostLayer{
		Hash:    fsLayer.Digest.String(),
		Path:    p,
		Headers: h,
	}, nil
}

func (c *Clair) getLayers(r *registry.Registry, repo, tag string, filterEmpty bool) (map[int]distribution.Descriptor, error) {
	ok := true
	// Get the manifest to pass to clair.
	mf, err := r.ManifestV2(repo, tag)
	if err != nil {
		ok = false
		c.Logf("couldn't retrieve manifest v2, falling back to v1")
	}

	filteredLayers := map[int]distribution.Descriptor{}

	// Filter out the empty layers.
	if ok {
		for i := 0; i < len(mf.Layers); i++ {
			if filterEmpty && IsEmptyLayer(mf.Layers[i].Digest) {
				continue
			} else {
				filteredLayers[len(mf.Layers)-i-1] = mf.Layers[i]
			}
		}

		return filteredLayers, nil
	}

	m, err := r.ManifestV1(repo, tag)
	if err != nil {
		return nil, fmt.Errorf("getting the v1 manifest for %s:%s failed: %v", repo, tag, err)
	}

	for i := 0; i < len(m.FSLayers); i++ {
		if filterEmpty && IsEmptyLayer(m.FSLayers[i].BlobSum) {
			continue
		} else {
			filteredLayers[i] = distribution.Descriptor{
				Digest: m.FSLayers[i].BlobSum,
			}
		}
	}

	return filteredLayers, nil
}
