package testutils

import (
	"context"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
)

// RegistryHelper implements methods to manipulate docker registry from test cases
type RegistryHelper struct {
	dcli *client.Client
	auth string
	addr string
}

// NewRegistryHelper returns RegistryHelper
func NewRegistryHelper(dcli *client.Client, username, password, addr string) (*RegistryHelper, error) {
	auth, err := constructRegistryAuth(username, password)
	if err != nil {
		return nil, err
	}
	return &RegistryHelper{dcli: dcli, auth: auth, addr: addr}, nil
}

// RefillRegistry adds images to a registry.
func (r *RegistryHelper) RefillRegistry(image string) error {
	if err := pullDockerImage(r.dcli, image); err != nil {
		return err
	}

	if err := r.dcli.ImageTag(context.Background(), image, r.addr+"/"+image); err != nil {
		return err
	}

	resp, err := r.dcli.ImagePush(context.Background(), r.addr+"/"+image, types.ImagePushOptions{
		RegistryAuth: r.auth,
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	fd, isTerm := term.GetFdInfo(os.Stdout)

	return jsonmessage.DisplayJSONMessagesStream(resp, os.Stdout, fd, isTerm, nil)
}
