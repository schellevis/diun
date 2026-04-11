package docker

import (
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
)

// RebuildContainer pulls the latest image for the given container, then stops,
// removes and recreates it with the same configuration. It returns the new
// container ID.
func (c *Client) RebuildContainer(containerID string) (string, error) {
	// Inspect current container to get its full configuration
	info, err := c.API.ContainerInspect(c.ctx, containerID)
	if err != nil {
		return "", errors.Wrapf(err, "cannot inspect container %s", containerID)
	}

	imageName := info.Config.Image

	// Pull the latest image
	rc, err := c.API.ImagePull(c.ctx, imageName, image.PullOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "cannot pull image %s", imageName)
	}
	// Drain and close the reader; Docker does not start the pull until the body is consumed
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()

	// Stop the running container (ignore error if already stopped)
	_ = c.API.ContainerStop(c.ctx, containerID, container.StopOptions{})

	// Remove the old container
	if err = c.API.ContainerRemove(c.ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return "", errors.Wrapf(err, "cannot remove container %s", containerID)
	}

	// Recreate container using the same config / host config / networking
	endpointCfg := make(map[string]*network.EndpointSettings)
	if info.NetworkSettings != nil {
		for netName, ep := range info.NetworkSettings.Networks {
			ep := ep
			endpointCfg[netName] = ep
		}
	}
	netCfg := &network.NetworkingConfig{EndpointsConfig: endpointCfg}

	resp, err := c.API.ContainerCreate(c.ctx, info.Config, info.HostConfig, netCfg, nil, info.Name)
	if err != nil {
		return "", errors.Wrapf(err, "cannot recreate container %s", info.Name)
	}

	// Start the new container
	if err = c.API.ContainerStart(c.ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", errors.Wrapf(err, "cannot start container %s", resp.ID)
	}

	return resp.ID, nil
}
