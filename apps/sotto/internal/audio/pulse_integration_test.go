//go:build integration

package audio

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListDevicesIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	devices, err := ListDevices(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, devices)
}
