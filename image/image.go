package image

import (
	"fmt"
)

// NAME is the name of the image that is embedded at compile time.
var NAME string

// SHA is the sha digest of the image that is embedded at compile time.
var SHA string

// Data returns the tarball image data that is embedded at compile time.
func Data() ([]byte, error) {
	data, err := Asset("image.tar")
	if err != nil {
		return nil, fmt.Errorf("getting bindata asset image.tar failed: %v", err)
	}

	return data, nil
}
